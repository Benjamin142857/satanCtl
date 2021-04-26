package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
)

var regStruct = regexp.MustCompile(`(?U)[ \t]*struct[ \t]+(?P<structName>[a-zA-Z_][0-9a-zA-Z_]*)[ \n\t]*{(?P<structCtx>(.|[\n])*)}`)
var regFunc = regexp.MustCompile(`(?U)[ \t]*func[ \t]+(?P<funcName>[a-zA-Z_][0-9a-zA-Z_]*)[ \n\t]*{[ \t\n]*req[ \t\n]*\((?P<req>(.|[\n])*)\)[ \t\n]*rsp[ \t\n]*\((?P<rsp>(.|[\n])*)\)[ \t\n]*}`)
var regStructField = regexp.MustCompile(
	"^[ \\t]*(?P<fieldName>[a-zA-Z_][0-9a-zA-Z_]*)" +
		"[ \\t]+(?P<fieldType>[a-zA-Z0-9_\\[\\]]+)" +
		"(?P<fieldFilter>[ \\t]+[a-zA-Z0-9,() \\t]*)?" +
		"(?P<fieldComment>[ \\t]*//[ \\t]*.*)?$",
)
var regMapType = regexp.MustCompile(`^map\[(?P<key>[a-z]+)\](?P<value>.+)$`)

type stProtocolType byte

const (
	Unknown stProtocolType = iota
	Byte
	Bool
	Int
	Long
	Float
	Double
	String
	List
	Map
	Struct
)

var stBaseTypeMap = map[string]stProtocolType{
	"byte":   Byte,
	"bool":   Bool,
	"int":    Int,
	"long":   Long,
	"float":  Float,
	"double": Double,
	"string": String,
}

type stProtoField struct {
	name          string
	dataType      stProtocolType
	subDataTypes  []stProtocolType
	subStructName string
	filters       []string
	comment       string
	defaultValue  string
}

type stProtoStruct struct {
	name      string
	comment   string
	fieldList []*stProtoField
}

type stProtoFunc struct {
	name    string
	comment string
	req     *stProtoStruct
	rsp     *stProtoStruct
}

type stProtoParser struct {
	directory     string
	serverName    string
	servantName   string
	fileText      string
	tgtFileText   string
	structNameMap map[string]bool
	structMap     map[string]*stProtoStruct
	funcList      []*stProtoFunc
}

func (psr *stProtoParser) parseOneStruct(structName string, structCtx string) (ps *stProtoStruct, err error) {
	if structCtx == "" {
		return nil, newStCtlError(fmt.Sprintf("struct %v is empty, it must have at least one field", structName))
	}

	if psr.structMap[structName] != nil {
		return nil, newStCtlError(fmt.Sprintf("struct %v is duplicated", structName))
	}

	ps = &stProtoStruct{name: structName, fieldList: make([]*stProtoField, 0)}
	for _, sFieldLine := range strings.Split(structCtx, "\n") {
		sFieldLine = strings.TrimSpace(sFieldLine)
		regStructFieldRes := regStructField.FindStringSubmatch(sFieldLine)
		if len(regStructFieldRes) < 5 {
			return nil, newStCtlError(fmt.Sprintf("line \"%v\" parse error", sFieldLine))
		} else {
			sFieldName := regStructFieldRes[1]
			sFieldType := regStructFieldRes[2]
			sFieldFilter := regStructFieldRes[3]
			sFieldComment := regStructFieldRes[4]
			dts, subStructName, err := psr.getStProtocolType(sFieldType)
			if err != nil {
				return nil, newStCtlError(fmt.Sprintf("struct %v parse error: field %v type \"%v\" error", structName, sFieldName, sFieldType))
			}

			pf := &stProtoField{
				name:          sFieldName,
				dataType:      dts[0],
				subDataTypes:  dts[1:],
				subStructName: subStructName,
				filters:       strings.Split(sFieldFilter, " "),
				comment:       sFieldComment,
				defaultValue:  "",
			}

			ps.fieldList = append(ps.fieldList, pf)
		}
	}
	return
}

func (psr *stProtoParser) parseStruct() error {
	regStructRes := regStruct.FindAllStringSubmatch(psr.fileText, -1)
	for _, structRes := range regStructRes {
		if len(structRes) < 4 {
			return newStCtlError("parse struct error")
		}
		structName := strings.TrimSpace(structRes[1])
		psr.structNameMap[structName] = true
	}
	for _, structRes := range regStructRes {
		if len(structRes) < 3 {
			return newStCtlError("parse struct error")
		}

		structName := strings.TrimSpace(structRes[1])
		structCtx := strings.TrimSpace(structRes[2])
		ps, err := psr.parseOneStruct(structName, structCtx)
		if err != nil {
			return err
		}
		psr.structMap[structName] = ps
	}
	return nil
}

func (psr *stProtoParser) parseFunc() error {
	regFuncRes := regFunc.FindAllStringSubmatch(psr.fileText, -1)
	for _, funcRes := range regFuncRes {
		if len(funcRes) < 6 {
			return newStCtlError("parse func error")
		}
		funcName := strings.TrimSpace(funcRes[1])
		sReq := strings.TrimSpace(funcRes[2])
		sRsp := strings.TrimSpace(funcRes[4])

		psReq, err := psr.parseOneStruct(fmt.Sprintf("%vReq", funcName), sReq)
		if err != nil {
			return err
		}
		psRsp, err := psr.parseOneStruct(fmt.Sprintf("%vRsp", funcName), sRsp)
		if err != nil {
			return err
		}

		pf := &stProtoFunc{
			name:    funcName,
			comment: "",
			req:     psReq,
			rsp:     psRsp,
		}
		psr.funcList = append(psr.funcList, pf)
	}
	return nil
}

func (psr *stProtoParser) getStProtocolType(s string) (dts []stProtocolType, structName string, err error) {
	if s == "" {
		err = newStCtlError("getStProtocolType error")
	} else if dt := stBaseTypeMap[s]; dt != Unknown {
		// base
		dts = append(dts, dt)
	} else if psr.structNameMap[s] {
		// struct
		dts = append(dts, Struct)
		structName = s
	} else if strings.Index(s, "[]") == 0 {
		// list
		subDts, subStruct, err := psr.getStProtocolType(s[2:])
		if err != nil {
			return nil, "", err
		}
		dts = append(dts, List)
		dts = append(dts, subDts...)
		structName = subStruct
	} else if regMapTypeRes := regMapType.FindStringSubmatch(s); len(regMapTypeRes) >= 3 {
		// map
		key := regMapTypeRes[1]
		value := regMapTypeRes[2]

		// key
		kdt := stBaseTypeMap[key]
		if kdt == Unknown {
			err = newStCtlError("getStProtocolType error")
			return
		}

		// value
		vdt, subStruct, err := psr.getStProtocolType(value)
		if err != nil {
			return nil, "", err
		}

		dts = append(dts, Map, kdt)
		dts = append(dts, vdt...)
		structName = subStruct
	} else {
		err = newStCtlError("getStProtocolType error")
	}
	return
}

func getStProtoFilesPath(directory string) (fileList []string, err error) {
	fileInfoList, err := ioutil.ReadDir(directory)
	if err != nil {
		return nil, err
	}

	for i := range fileInfoList {
		if fileName := fileInfoList[i].Name(); !fileInfoList[i].IsDir() && strings.LastIndex(fileName, ".stproto") == len(fileName)-8 {
			fileList = append(fileList, path.Join(directory, fileName))
		}
	}
	return
}

func parseStProtoFile(filePath string) (*stProtoParser, error) {
	fileName := path.Base(filePath)
	fileDir :=  path.Dir(filePath)
	nowDir, _ := os.Getwd()
	buff, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	psr := &stProtoParser{
		directory:     fileDir,
		serverName:    getServerNameFromPath(fileDir, nowDir),
		servantName:   strings.TrimSuffix(fileName, path.Ext(fileName)),
		fileText:      string(buff),
		tgtFileText:   "",
		structNameMap: make(map[string]bool),
		structMap:     make(map[string]*stProtoStruct),
		funcList:      make([]*stProtoFunc, 0),
	}
	return psr, nil
}

func getServerNameFromPath(fileDir, nowDir string) string {
	if path.IsAbs(fileDir) {
		return path.Base(fileDir)
	} else if fileDir == "" {
		return path.Base(nowDir)
	}

	if strings.Index(fileDir, "..") == 0 {
		sp := strings.Split(fileDir, string(os.PathSeparator))
		return getServerNameFromPath(path.Join(sp[1:]...), path.Dir(nowDir))
	} else if strings.Index(fileDir, ".") == 0 {
		sp := strings.Split(fileDir, string(os.PathSeparator))
		return getServerNameFromPath(path.Join(sp[1:]...), nowDir)
	} else {
		return path.Base(fileDir)
	}
}
