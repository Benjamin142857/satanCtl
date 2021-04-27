package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
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
		fmt.Printf("parsing %v...\n", path.Base(filePath))
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
		if err := psr.toGoFile(); err != nil {
			fmt.Println(err)
			return
		}
	}

	fmt.Println("st2go finish >>>>>>>>>>>>>>>>>>>>>")
}

var toGoDataTypeStrMap = map[stProtocolType]string{
	Byte:   "Byte",
	Bool:   "Bool",
	Int:    "Int",
	Long:   "Long",
	Float:  "Float",
	Double: "Double",
	String: "String",
	List:   "List",
	Map:    "Map",
	Struct: "Struct",
}

var toGoDataTypeGoMap = map[stProtocolType]string{
	Byte:   "byte",
	Bool:   "bool",
	Int:    "int",
	Long:   "int64",
	Float:  "float32",
	Double: "float64",
	String: "string",
	List:   "[]%v",
	Map:    "map[%v]%v",
	Struct: "*%v",
}

var toGoDefaultValueMap = map[stProtocolType]string{
	Byte:   "byte(0)",
	Bool:   "false",
	Int:    "0",
	Long:   "int64(0)",
	Float:  "float32(0)",
	Double: "float64(0)",
	String: "\"\"",
	List:   "make(%v, 0)",
	Map:    "make(%v)",
	Struct: "nil",
}

func (psr *stProtoParser) toGoFile() error {
	// Header
	psr.toGoWriteHeader()
	// struct
	for _, st := range psr.structMap {
		psr.tgtFileText += st.toGoWriteStruct()
		psr.tgtFileText += st.toGoWriteFuncWriteDataBuf()
		psr.tgtFileText += st.toGoWriteFuncReadDataBuf()
		psr.tgtFileText += st.toGoWriteFuncNewPerson()
		psr.tgtFileText += "\n"
	}
	// servant struct
	// servant func



	filePath := path.Join(psr.directory, fmt.Sprintf("%v.stproto.go", psr.servantName))
	if err := ioutil.WriteFile(filePath, []byte(psr.tgtFileText), 0666); err != nil {
		return err
	}
	cmd := exec.Command("go", "fmt", filePath)
	_ = cmd.Run()
	return nil
}

func (psr *stProtoParser) toGoWriteHeader() {
	psr.tgtFileText += fmt.Sprintf("package %v\n\n", psr.serverName)

	psr.tgtFileText += "import (\n"
	psr.tgtFileText += "\t\"satanGo/satan/errors\"\n"
	psr.tgtFileText += "\t\"satanGo/satan/protocol\"\n"
	psr.tgtFileText += ")\n\n"
}
func (psr *stProtoParser) toGoWriteServantStruct() {
	psr.tgtFileText += fmt.Sprintf("package %v\n\n", psr.serverName)

	psr.tgtFileText += "import (\n"
	psr.tgtFileText += "\t\"satanGo/satan/errors\"\n"
	psr.tgtFileText += "\t\"satanGo/satan/protocol\"\n"
	psr.tgtFileText += ")\n\n"
}

func (ps *stProtoStruct) toGoWriteStruct() string {
	var ret string
	ret += fmt.Sprintf("type %v struct {\n", upperFirstChar(ps.name))
	for _, pf := range ps.fieldList {
		ret += fmt.Sprintf(
			"\t%v %v `json:\"%v\"`\n",
			upperFirstChar(pf.name),
			pf.toGoGetDataTypeStr(pf.dataType, pf.subDataTypes),
			pf.name,
		)
	}
	ret += "}\n"

	return ret
}

func (ps *stProtoStruct) toGoWriteFuncWriteDataBuf() string {
	var ret string
	ret += fmt.Sprintf("func (st *%v) WriteDataBuf(bf *protocol.StBuffer) error {\n", ps.name)
	// length
	ret += fmt.Sprintf("\tif err := bf.WriteStructLength(%v); err != nil {\n", len(ps.fieldList))
	ret += "\t\treturn err"
	ret += "\t}\n\n"

	// field
	for tg, pf := range ps.fieldList {
		// field tag
		ret += fmt.Sprintf("\tif err := bf.WriteTag(%v); err != nil  {\n", tg)
		ret += "\t\treturn err\n"
		ret += "\t}\n"

		// field datatype
		ret += fmt.Sprintf("\tif err := bf.WriteDataType(protocol.%v); err != nil {\n", toGoDataTypeStrMap[pf.dataType])
		ret += "\t\treturn err\n"
		ret += "\t}\n"

		// field dataBuf
		ret += pf.toGoWriteDataBuf(1, pf.dataType, pf.subDataTypes, "st."+upperFirstChar(pf.name))
		ret += "\n"
	}

	ret += "\treturn nil\n"
	ret += "}\n"
	return ret
}
func (ps *stProtoStruct) toGoWriteFuncReadDataBuf() string {
	var ret string
	ret += fmt.Sprintf("func (st *%v) ReadDataBuf(bf *protocol.StBuffer) error {\n", ps.name)
	// length
	ret += "\tl, err := bf.ReadStructLength()\n"
	ret += "\tif err != nil {\n"
	ret += "\t\treturn err\n"
	ret += "\t}\n\n"

	// field
	ret += "\tfor i := byte(0); i < l; i++ {\n"
	ret += "\t\ttg, err := bf.ReadTag()\n"
	ret += "\t\tif err != nil {\n"
	ret += "\t\t\treturn err\n"
	ret += "\t\t}\n"
	ret += "\t\tif _, err := bf.ReadDataType(); err != nil {\n"
	ret += "\t\t\treturn err\n"
	ret += "\t\t}\n\n"

	ret += "\t\tswitch tg {\n"
	for tg, pf := range ps.fieldList {
		ret += fmt.Sprintf("\t\tcase byte(%v):\n", tg)
		ret += pf.toGoReadDataBuf(1, pf.dataType, pf.subDataTypes, "d")
		ret += fmt.Sprintf("\t\t\tst.%v = d1\n", upperFirstChar(pf.name))
	}
	ret += "\t\t}\n\n"
	ret += "\t}\n"

	ret += "\treturn nil\n"
	ret += "}\n"
	return ret
}
func (ps *stProtoStruct) toGoWriteFuncNewPerson() string {
	var ret string
	stName := upperFirstChar(ps.name)
	ret += fmt.Sprintf("func New%v() *%v {\n", stName, stName)
	ret += fmt.Sprintf("\treturn &%v{\n", stName)
	for _, pf := range ps.fieldList {
		ret += fmt.Sprintf("\t\t%v: %v,\n", upperFirstChar(pf.name), pf.toGoGetDefaultValue())
	}
	ret += "\t}\n"
	ret += "}\n"
	return ret
}

func (pf *stProtoField) toGoGetDefaultValue() string {
	if pf.defaultValue != "" {
		return pf.defaultValue
	}

	switch pf.dataType {
	case Byte, Bool, Int, Long, Float, Double, String:
		return toGoDefaultValueMap[pf.dataType]
	case List, Map:
		return fmt.Sprintf(toGoDefaultValueMap[pf.dataType], pf.toGoGetDataTypeStr(pf.dataType, pf.subDataTypes))
	default:
		return fmt.Sprintf(toGoDefaultValueMap[pf.dataType])
	}
}
func (pf *stProtoField) toGoGetDataTypeStr(dt stProtocolType, sDts []stProtocolType) string {
	switch dt {
	case Byte, Bool, Int, Long, Float, Double, String:
		return toGoDataTypeGoMap[dt]
	case List:
		return fmt.Sprintf(toGoDataTypeGoMap[dt], pf.toGoGetDataTypeStr(sDts[0], sDts[1:]))
	case Map:
		return fmt.Sprintf(toGoDataTypeGoMap[dt], pf.toGoGetDataTypeStr(sDts[0], []stProtocolType{}), pf.toGoGetDataTypeStr(sDts[1], sDts[2:]))
	default:
		return fmt.Sprintf(toGoDataTypeGoMap[dt], upperFirstChar(pf.subStructName))
	}
}
func (pf *stProtoField) toGoWriteDataBuf(tbIdx int, tp stProtocolType, sTps []stProtocolType, forStr string) string {
	var ret string
	var tb string
	for i:=0; i<tbIdx; i++ {
		tb += "\t"
	}

	switch tp {
	case Byte, Bool, Int, Long, Float, Double, String:
		ret += fmt.Sprintf("%vif err := bf.WriteDataBuf(protocol.%v, %v); err != nil {\n", tb, toGoDataTypeStrMap[tp], forStr)
		ret += fmt.Sprintf("\t%vreturn err\n", tb)
		ret += fmt.Sprintf("%v}\n", tb)
	case List:
		// elem dataType
		ret += fmt.Sprintf("%vif err := bf.WriteDataType(protocol.%v); err != nil {\n", tb, toGoDataTypeStrMap[sTps[0]])
		ret += fmt.Sprintf("\t%vreturn err\n", tb)
		ret += fmt.Sprintf("%v}\n", tb)
		// length
		ret += fmt.Sprintf("%vif err := bf.WriteLength(len(%v)); err != nil {\n", tb, forStr)
		ret += fmt.Sprintf("\t%vreturn err\n", tb)
		ret += fmt.Sprintf("%v}\n", tb)
		// for [dataBuf]...
		if sTps[0] == Byte {
			ret += fmt.Sprintf("%vif err := bf.WriteBytes(%v); err != nil {\n", tb, forStr)
			ret += fmt.Sprintf("\t%vreturn err\n", tb)
			ret += fmt.Sprintf("%v}\n", tb)
		} else {
			ret += fmt.Sprintf("%vfor _, e%v := range %v {\n", tb, tbIdx, forStr)
			ret += pf.toGoWriteDataBuf(tbIdx+1, sTps[0], sTps[1:], fmt.Sprintf("e%v", tbIdx))
			ret += fmt.Sprintf("%v}\n", tb)
		}

	case Map:
		// key dataType
		ret += fmt.Sprintf("%vif err := bf.WriteDataType(protocol.%v); err != nil {\n", tb, toGoDataTypeStrMap[sTps[0]])
		ret += fmt.Sprintf("\t%vreturn err\n", tb)
		ret += fmt.Sprintf("%v}\n", tb)
		// value dataType
		ret += fmt.Sprintf("%vif err := bf.WriteDataType(protocol.%v); err != nil {\n", tb, toGoDataTypeStrMap[sTps[1]])
		ret += fmt.Sprintf("\t%vreturn err\n", tb)
		ret += fmt.Sprintf("%v}\n", tb)
		// length
		ret += fmt.Sprintf("%vif err := bf.WriteLength(len(%v)); err != nil {\n", tb, forStr)
		ret += fmt.Sprintf("\t%vreturn err\n", tb)
		ret += fmt.Sprintf("%v}\n", tb)
		// for [dataBuf]...
		ret += fmt.Sprintf("%vfor k%v, v%v := range %v {\n", tb, tbIdx, tbIdx, forStr)
		ret += pf.toGoWriteDataBuf(tbIdx+1, sTps[0], []stProtocolType{}, fmt.Sprintf("k%v", tbIdx))
		ret += pf.toGoWriteDataBuf(tbIdx+1, sTps[1], sTps[2:], fmt.Sprintf("v%v", tbIdx))
		ret += fmt.Sprintf("%v}\n", tb)
	case Struct:
		ret += fmt.Sprintf("%vif err := %v.WriteDataBuf(bf); err != nil {\n", tb, forStr)
		ret += fmt.Sprintf("\t%vreturn err\n", tb)
		ret += fmt.Sprintf("%v}\n", tb)
	}

	return ret
}
func (pf *stProtoField) toGoReadDataBuf(tbIdx int, tp stProtocolType, sTps []stProtocolType, forStr string) string {
	var ret string
	var tb string
	for i:=0; i<tbIdx+2; i++ {
		tb += "\t"
	}
	varName := fmt.Sprintf("%v%v", forStr, tbIdx)

	switch tp {
	case Byte, Bool, Int, Long, Float, Double, String:
		ret += fmt.Sprintf("%v_%v, err := bf.ReadDataBuf(protocol.%v)\n", tb, varName, toGoDataTypeStrMap[tp])
		ret += fmt.Sprintf("%vif err != nil {\n", tb)
		ret += fmt.Sprintf("\t%vreturn err\n", tb)
		ret += fmt.Sprintf("%v}\n", tb)
		ret += fmt.Sprintf("%v%v, ok := _%v.(%v)\n", tb, varName, varName, toGoDataTypeGoMap[tp])
		ret += fmt.Sprintf("%vif !ok {\n", tb)
		ret += fmt.Sprintf("\t%vreturn errors.NewStError(1004)\n", tb)
		ret += fmt.Sprintf("%v}\n", tb)
	case List:

		// elem dataType
		ret += fmt.Sprintf("%vif _, err := bf.ReadDataType(); err != nil {\n", tb)
		ret += fmt.Sprintf("\t%vreturn err\n", tb)
		ret += fmt.Sprintf("%v}\n", tb)
		// length
		ret += fmt.Sprintf("%vl%v, err := bf.ReadLength()\n", tb, tbIdx)
		ret += fmt.Sprintf("%vif err != nil {\n", tb)
		ret += fmt.Sprintf("\t%vreturn err\n", tb)
		ret += fmt.Sprintf("%v}\n", tb)
		// for [dataBuf]...
		if sTps[0] == Byte {
			ret += fmt.Sprintf("%v%v, err := bf.ReadBytes(l%v)\n", tb, varName, tbIdx)
			ret += fmt.Sprintf("%vif err != nil {\n", tb)
			ret += fmt.Sprintf("\t%vreturn err\n", tb)
			ret += fmt.Sprintf("%v}\n", tb)
		} else {
			// init
			ret += fmt.Sprintf("%v%v := make(%v, l%v)\n", tb, varName, pf.toGoGetDataTypeStr(tp, sTps), tbIdx)
			ret += fmt.Sprintf("%vfor i%v:=0; i%v<l%v; i%v++ {\n", tb, tbIdx, tbIdx, tbIdx, tbIdx)
			ret += pf.toGoReadDataBuf(tbIdx+1, sTps[0], sTps[1:], "e")
			ret += fmt.Sprintf("\t%v%v[i%v] = e%v\n", tb, varName, tbIdx, tbIdx+1)
			ret += fmt.Sprintf("%v}\n", tb)
		}
	case Map:
		// init
		ret += fmt.Sprintf("%v%v := make(%v)\n", tb, varName, pf.toGoGetDataTypeStr(tp, sTps))
		// key dataType
		ret += fmt.Sprintf("%vif _, err := bf.ReadDataType(); err != nil {\n", tb)
		ret += fmt.Sprintf("\t%vreturn err\n", tb)
		ret += fmt.Sprintf("%v}\n", tb)
		// value dataType
		ret += fmt.Sprintf("%vif _, err := bf.ReadDataType(); err != nil {\n", tb)
		ret += fmt.Sprintf("\t%vreturn err\n", tb)
		ret += fmt.Sprintf("%v}\n", tb)
		// length
		ret += fmt.Sprintf("%vl%v, err := bf.ReadLength()\n", tb, tbIdx)
		ret += fmt.Sprintf("%vif err != nil {\n", tb)
		ret += fmt.Sprintf("\t%vreturn err\n", tb)
		ret += fmt.Sprintf("%v}\n", tb)
		// for [dataBuf]...
		ret += fmt.Sprintf("%vfor i%v:=0; i%v<l%v; i%v++ {\n", tb, tbIdx, tbIdx, tbIdx, tbIdx)
		ret += pf.toGoReadDataBuf(tbIdx+1, sTps[0], []stProtocolType{}, "k")
		ret += pf.toGoReadDataBuf(tbIdx+1, sTps[1], sTps[2:], "v")
		ret += fmt.Sprintf("\t%v%v[k%v] = v%v\n", tb, varName, tbIdx+1, tbIdx+1)
		ret += fmt.Sprintf("%v}\n", tb)
	case Struct:
		// init
		ret += fmt.Sprintf("%v%v := New%v()\n", tb, varName, upperFirstChar(pf.subStructName))
		// ReadDataBuf
		ret += fmt.Sprintf("%vif err := %v.ReadDataBuf(bf); err != nil {\n", tb, varName)
		ret += fmt.Sprintf("\t%vreturn err\n", tb)
		ret += fmt.Sprintf("%v}\n", tb)
	}

	return ret
}

func upperFirstChar(s string) string {
	return strings.ToUpper(s)[0:1]+s[1:]
}