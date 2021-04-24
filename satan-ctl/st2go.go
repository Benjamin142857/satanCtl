package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"path"
	"regexp"
	"strings"
)

var St2Go = &St2GoCommand{}

type St2GoCommand struct {
	directory string
}

func (c *St2GoCommand) ParseArgs(args []string) error {
	fs := flag.NewFlagSet("st2go", flag.ContinueOnError)
	directory := fs.String("d", "./", "stproto file directory")

	if err := fs.Parse(args); err != nil {
		return err
	}

	c.directory = *directory
	return nil
}

func (c *St2GoCommand) Description() string {
	return "\n\t\t基于 stproto 文件生成 satanGo 服务接口依赖." +
		"\n\t\tGenerate Satango service interface dependencies based on stproto files."
}

func (c *St2GoCommand) Exec() {
	fileList, err := getStProtoFilesPath(c.directory)
	if err != nil {
		fmt.Println(err)
		return
	}

	var psrList []*stProtoParser
	for _, filePath := range fileList {
		psr, err := parseStProtoFile(filePath)
		if err != nil {
			fmt.Println(err)
			return
		}
		if err := psr.parseStruct(); err != nil {
			fmt.Println(err)
			return
		}
		if err := psr.parseFunc(); err != nil {
			fmt.Println(err)
			return
		}
		psrList = append(psrList, psr)
	}

	for _, psr := range psrList {
		psr.toGoFile()
	}
}

var regStruct = regexp.MustCompile(`(?U)[ \t]*struct[ \t]+(?P<structName>[a-zA-Z_][0-9a-zA-Z_]*)[ \t]*{(?P<structCtx>(.|[\n])*)}`)
var regStructField = regexp.MustCompile(
	"^[ \t]*(?P<fieldName>[a-zA-Z_][0-9a-zA-Z_]*)" +
		"[ \t]+(?P<fieldType>[a-zA-Z0-9]+)" +
		"(?P<fieldFilter>[ \t]+[a-zA-Z0-9,() \t]*)?" +
		"(?P<fieldComment>[ \t]*//[ \t]*.*)?$",
)

type st2goError struct {
	errMsg string
}

func (e *st2goError) Error() string {
	return e.errMsg
}
func newSt2goError(errMsg string) *st2goError {
	return &st2goError{errMsg: errMsg}
}

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

var StBaseTypeMap = map[string]stProtocolType{
	"byte":  Byte,
	"bool":  Bool,
	"int":  Int,
	"long":  Long,
	"float":  Float,
	"double":  Double,
	"string":  String,
}

type stProtoFieldFilter interface{}

type stProtoField struct {
	name         string
	dataType     stProtocolType
	subDataTypes []stProtocolType
	fieldFilters []stProtoFieldFilter
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
	directory  string
	serverName string
	fileText   string
	structMap  map[string]*stProtoStruct
	funcList   []*stProtoFunc
}

func (psr *stProtoParser) parseStruct() error {
	regStructRes := regStruct.FindAllStringSubmatch(psr.fileText, -1)
	for _, structRes := range regStructRes {
		if len(structRes) < 3 {
			return newSt2goError("parse struct error")
		}

		structName := strings.TrimSpace(structRes[1])
		structCtx := strings.TrimSpace(structRes[2])

		if structCtx == "" {
			return newSt2goError(fmt.Sprintf("struct %v is empty, it must have at least one field", structName))
		}

		if psr.structMap[structName] != nil {
			return newSt2goError(fmt.Sprintf("struct %v is duplicated", structName))
		}

		ps := &stProtoStruct{name: structName, fieldList: make([]*stProtoField, 0)}
		fmt.Println(structName, ">>>>>>>>>>>>>>>>>>>>>>>>")
		for _, sFieldLine := range strings.Split(structCtx, "\n") {
			sFieldLine = strings.TrimSpace(sFieldLine)
			regStructFieldRes := regStructField.FindStringSubmatch(sFieldLine)
			if len(regStructFieldRes) < 5 {
				return newSt2goError(fmt.Sprintf("line \"%v\" parse error", sFieldLine))
			} else {
				pf := &stProtoField{
					name: regStructFieldRes[1],

				}
				ps.fieldList = append(ps.fieldList, pf)

				ps.fieldList
				psr.structMap[structName] = ps
				fmt.Printf("<%v>\t", regStructFieldRes[1])
				fmt.Printf("<%v>\t", regStructFieldRes[2])
				fmt.Printf("<%v>\t", regStructFieldRes[3])
				fmt.Printf("<%v>\n", regStructFieldRes[4])
			}
		}
		psr.structMap[structName] = ps
	}
	return nil
}

func (psr *stProtoParser) parseFunc() error {
	return nil
}

func (psr *stProtoParser) toGoFile() {

}

func getStProtocolType(s string) (dts []stProtocolType, err error) {
	if s=="" {
		err = newSt2goError("getStProtocolType error")
	} else if dt := StBaseTypeMap[s]; dt != Unknown {
		dts = append(dts, dt)
	} else if strings.Index(s, "[]") == 0 {
		subDts, err := getStProtocolType(s[2:])
		if err != nil {
			return
		}
		dts = append(dts, List)
		dts = append(dts, subDts...)
	} else if
	return
}

func getStProtoFilesPath(directory string) (fileList []string, err error) {
	fileInfoList, err := ioutil.ReadDir(directory)
	if err != nil {
		return nil, err
	}

	for i := range fileInfoList {
		if !fileInfoList[i].IsDir() {
			fileList = append(fileList, path.Join(directory, fileInfoList[i].Name()))
		}
	}
	return
}

func parseStProtoFile(filePath string) (*stProtoParser, error) {
	fileName := path.Base(filePath)
	buff, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	psr := &stProtoParser{
		directory:  path.Dir(filePath),
		serverName: strings.TrimSuffix(fileName, path.Ext(fileName)),
		fileText:   string(buff),
		structMap: 	make(map[string]*stProtoStruct),
		funcList:   make([]*stProtoFunc, 0),
	}
	return psr, nil
}
