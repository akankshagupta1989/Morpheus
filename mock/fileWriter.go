package mock

import (
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"os"
	"os/exec"
	"strings"
)

type MockedObject struct {
	reflectNum     int
	toImport       map[string]int
	fileName       string
	importDecls    string
	interfaceDecls string
	structDecls    string
	functionDecls  string
}

var builtinTypes = map[string]bool{
	"ComplexType": true,
	"FloatType":   true,
	"IntegerType": true,
	"Type":        true,
	"Type1":       true,
	"bool":        true,
	"byte":        true,
	"complex128":  true,
	"complex64":   true,
	"error":       true,
	"float32":     true,
	"float64":     true,
	"int":         true,
	"int16":       true,
	"int32":       true,
	"int64":       true,
	"int8":        true,
	"rune":        true,
	"string":      true,
	"uint":        true,
	"uint16":      true,
	"uint32":      true,
	"uint64":      true,
	"uint8":       true,
	"uintptr":     true,
}

func typeFieldList(fl *ast.FieldList, optParen bool) string {

	var list []string

	if fl == nil {
		return ""
	}
	for _, field := range fl.List {
		cnt := len(field.Names)
		if cnt == 0 {
			cnt = 1
		}

		for i := 0; i < cnt; i++ {
			list = append(list, typeString(field.Type))
		}
	}

	if optParen {
		if len(list) == 1 {
			return list[0]
		}

		return "(" + strings.Join(list, ", ") + ")"
	}

	return strings.Join(list, ", ")
}

func typeString(typ ast.Expr) string {

	switch specific := typ.(type) {

	case *ast.Ident:
		return specific.Name
	case *ast.StarExpr:
		return "*" + typeString(specific.X)
	case *ast.ArrayType:
		if specific.Len == nil {
			return "[]" + typeString(specific.Elt)
		} else {
			var l string

			switch ls := specific.Len.(type) {
			case *ast.BasicLit:
				l = ls.Value
			default:
				panic(fmt.Sprintf("unable to figure out array length: %#v", specific.Len))
			}
			return "[" + l + "]" + typeString(specific.Elt)
		}
	case *ast.SelectorExpr:
		if ident, ok := specific.X.(*ast.Ident); ok {
			return ident.Name + "." + specific.Sel.Name
		} else {
			panic(fmt.Sprintf("strange selector expr encountered: %#v", specific))
		}
	case *ast.InterfaceType:
		if len(specific.Methods.List) == 0 {
			return "interface{}"
		} else {
			panic(fmt.Sprintf("unable to handle this interface type: %#v", specific))
		}
	case *ast.MapType:
		return "map[" + typeString(specific.Key) + "]" + typeString(specific.Value)
	case *ast.Ellipsis:
		return "..." + typeString(specific.Elt)
	case *ast.FuncType:
		return "func(" + typeFieldList(specific.Params, false) + ") " + typeFieldList(specific.Results, true)
	case *ast.ChanType:
		switch specific.Dir {
		case ast.SEND:
			return "chan<- " + typeString(specific.Value)
		case ast.RECV:
			return "<-chan " + typeString(specific.Value)
		default:
			return "chan " + typeString(specific.Value)
		}
	default:
		panic(fmt.Sprintf("unable to handle type: %#v", typ))
	}
	return ""
}

func genList(list *ast.FieldList, addNames bool) ([]string, []string, []string) {

	var (
		params []string
		names  []string
		types  []string
	)

	if list == nil {
		return params, names, types
	}

	if !addNames {
		for _, param := range list.List {
			if len(param.Names) > 1 {
				addNames = true
				break
			}
		}
	}

	for idx, param := range list.List {
		ts := typeString(param.Type)

		var pname string

		if addNames {
			if len(param.Names) == 0 {
				pname = fmt.Sprintf("_a%d", idx)
				names = append(names, pname)
				types = append(types, ts)
				params = append(params, fmt.Sprintf("%s %s", pname, ts))

				continue
			}

			for _, name := range param.Names {
				pname = name.Name
				names = append(names, pname)
				types = append(types, ts)
				params = append(params, fmt.Sprintf("%s %s", pname, ts))
			}
		} else {
			names = append(names, "")
			types = append(types, ts)
			params = append(params, ts)
		}
	}

	return names, types, params
}

func WriteExportedContent(f FileInfo) {

	mockObj := InitMockedObject(f.filePath)
	importArr := make([][]string, 0)

	ast.Inspect(f.fileContent, func(n ast.Node) bool {

		switch nType := n.(type) {

		case *ast.FuncDecl:
			if nType.Name.IsExported() {
				mockObj.GenerateFuncCode(nType)
			}

		case *ast.GenDecl:

			for _, spec := range nType.Specs {
				typespec, ok := spec.(*ast.TypeSpec)
				if ok {
					_, ok := typespec.Type.(*ast.InterfaceType)
					if ok {
						mockObj.GenerateInterfaceCode(nType, f.fset, f.lines, typespec.Name.Name)
					}

					_, ok = typespec.Type.(*ast.StructType)
					if ok {
						mockObj.GenerateStructCode(nType, f.fset, f.lines, typespec.Name.Name)
					}
				}
				importSpec, ok := spec.(*ast.ImportSpec)
				if ok {
					import0 := make([]string, 2)
					import0[0] = importSpec.Path.Value
					if importSpec.Name != nil && importSpec.Name.Name != "" {
						import0[1] = importSpec.Name.Name
					} else {
						import0[1] = ""
					}

					importArr = append(importArr, import0)
				}
			}
		}
		return true
	})

	mockObj.GenerateImportCode(importArr)
	mockObj.writeToFile(f.parentDir)
}

func (m *MockedObject) GenerateImportCode(importArr [][]string) {

	toWrite := fmt.Sprintf("import ( \n \"fmt\"\n\"encoding/json\"\n")
	if m.reflectNum > 0 {
		toWrite = fmt.Sprintf("%s \"reflect\"\n", toWrite)
	}

	for importUsed, _ := range m.toImport {
		for _, importX := range importArr {

			toSearchImportPkg := fmt.Sprintf("/%s\"", importUsed)

			if importX[1] == importUsed {
				toWrite = fmt.Sprintf("%s %s %s\n", toWrite, importX[1], importX[0])
			} else if strings.HasSuffix(importX[0], toSearchImportPkg) {
				toWrite = fmt.Sprintf("%s %s\n", toWrite, importX[0])
			} else if importX[0] == fmt.Sprintf("\"%s\"", importUsed) {
				toWrite = fmt.Sprintf("%s %s\n", toWrite, importX[0])
			}
		}
	}

	if len(toWrite) > 0 {
		toWrite = fmt.Sprintf("%s \n)\n", toWrite)
	}

	m.importDecls = toWrite
}

func (m *MockedObject) GenerateStructCode(structType *ast.GenDecl, fset *token.FileSet, lines []string, name string) {

	startToken := structType.Pos()
	endToken := structType.End()
	startLine := fset.File(startToken).Line(startToken)
	endLine := fset.File(endToken).Line(endToken)
	toWrite := lines[startLine : endLine-1]

	if len(toWrite) > 0 {
		str := fmt.Sprintf("type %s struct { \n %s \n } \n", name, strings.Join(toWrite, "\n"))
		m.structDecls += str
	}
}

func (m *MockedObject) GenerateInterfaceCode(interfaceType *ast.GenDecl, fset *token.FileSet, lines []string, name string) {

	startToken := interfaceType.Pos()
	endToken := interfaceType.End()
	startLine := fset.File(startToken).Line(startToken)
	endLine := fset.File(endToken).Line(endToken)
	toWrite := lines[startLine : endLine-1]

	if len(toWrite) > 0 {
		str := fmt.Sprintf("type %s interface { \n %s \n } \n", name, strings.Join(toWrite, "\n"))
		m.interfaceDecls += str
	}

}

func (m *MockedObject) GenerateFuncCode(funcType *ast.FuncDecl) {

	ftype := funcType.Type
	objectName, _, object := genList(funcType.Recv, true)
	_, paramTypes, params := genList(ftype.Params, true)
	_, returntypes, _ := genList(ftype.Results, false)

	for _, paramType := range paramTypes {
		if strings.Contains(paramType, ".") {
			imp := strings.Split(paramType, ".")
			str := stripSpecialCharsinPrefix(imp[0])
			m.toImport[str] = 1
		}
	}

	for _, returnType := range returntypes {
		if strings.Contains(returnType, ".") {
			imp := strings.Split(returnType, ".")
			str := stripSpecialCharsinPrefix(imp[0])
			m.toImport[str] = 1
		}
	}

	toWrite := "func"
	if len(objectName) > 0 {
		toWrite = fmt.Sprintf("%s (%s) ", toWrite, string(object[0])) //writing object on which func is defined
	}
	parameters := strings.Join(params, ", ")       // func params
	returnTypes := strings.Join(returntypes, ", ") // func return types
	toWrite = fmt.Sprintf("%s %s (%s) (%s) {\n", toWrite, funcType.Name.Name, parameters, returnTypes)
	toWrite = fmt.Sprintf("%s jsonData := ServicesMap[\"%s\"].([]%sStruct)\n", toWrite, funcType.Name.Name, funcType.Name.Name)

	for i := 0; i < len(returntypes); i++ {
		toWrite = fmt.Sprintf("%s var return%d %s\n", toWrite, i, string(returntypes[i]))
	}

	toWrite = fmt.Sprintf("%s for i := 0; i < len(jsonData); i++ {\n elem := jsonData[i]\n inp := elem.Input\n outp := elem.Output\n", toWrite)
	result := ""

	for i := 0; i < len(params); i++ {
		if i > 0 {
			result = result + " && "
		}
		param := strings.Split(params[i], " ")
		_, isBuiltin := builtinTypes[param[1]]
		if isBuiltin {
			result = fmt.Sprintf("%s (%s == inp.%s)", result, param[0], strings.Title(param[0]))
		} else {
			result = fmt.Sprintf("%s reflect.DeepEqual(%s,inp.%s)", result, param[0], strings.Title(param[0]))
			m.reflectNum++
		}
	}

	toWrite = fmt.Sprintf("%s result := %s \n", toWrite, result)

	toWrite = fmt.Sprintf("%s if result == true {\n", toWrite)

	for i := 0; i < len(params); i++ {
		param := strings.Split(params[i], " ")

		if string(param[1][0]) == "*" {
			toWrite = fmt.Sprintf("%s *%s = *outp.%s\n", toWrite, string(param[0]), strings.Title(string(param[0])))
		}
	}

	for i := 0; i < len(returntypes); i++ {
		toWrite = fmt.Sprintf("%s return%d = outp.Return%d\n", toWrite, i, i)
	}

	toWrite = toWrite + "}\n}"

	returnVars := make([]string, 0)

	for i := 0; i < len(returntypes); i++ {
		returnVars = append(returnVars, fmt.Sprintf("return%d", i))
	}

	toReturn := strings.Join(returnVars, ", ")

	toWrite = fmt.Sprintf("%s\n return %s\n", toWrite, toReturn)

	toWrite = toWrite + "}"

	toWriteStruct := GenerateFunctionStruct(fmt.Sprintf("%sStruct", funcType.Name.Name), params, returntypes)

	m.structDecls = fmt.Sprintf("%s\n%s", m.structDecls, toWriteStruct)
	m.functionDecls = fmt.Sprintf("%s\n%s", m.functionDecls, toWrite)

}

func GenerateFunctionStruct(funcName string, params, returntypes []string) string {

	toWrite := fmt.Sprintf("type %s struct {\n", funcName)

	returnVarsInStruct := ""
	for i := 0; i < len(returntypes); i++ {
		returnVarsInStruct = fmt.Sprintf("%s Return%d %s `json:\"Return%d\"`\n", returnVarsInStruct, i, returntypes[i], i)
	}

	inputVarsInStruct := ""
	for i := 0; i < len(params); i++ {

		param := strings.Split(params[i], " ")

		if string(param[1][0]) == "*" {
			returnVarsInStruct = fmt.Sprintf("%s %s %s `json:\"%s\"`\n", returnVarsInStruct, strings.Title(param[0]), param[1], param[0])
		}

		inputVarsInStruct = fmt.Sprintf("%s %s %s `json:\"%s\"`\n", inputVarsInStruct, strings.Title(param[0]), param[1], param[0])

	}

	toWrite = fmt.Sprintf("%s Input struct { \n %s \n} \n", toWrite, inputVarsInStruct)

	if len(returntypes) > 0 {
		toWrite = fmt.Sprintf("%s Output struct { \n %s }\n", toWrite, returnVarsInStruct)
	}
	toWrite = fmt.Sprintf("%s } \n", toWrite)

	return toWrite
}

func InitMockedObject(fileName string) *MockedObject {
	return &MockedObject{
		fileName: fileName,
		toImport: make(map[string]int),
	}
}

func (m *MockedObject) writeToFile(packageName string) {

	tagName := fmt.Sprintf("// +build %smock\n", packageName)
	toWrite := fmt.Sprintf("%s\npackage %s\n%s\n%s\n%s\n%s", tagName, packageName, m.importDecls, m.interfaceDecls, m.structDecls, m.functionDecls)
	fileName := strings.Replace(m.fileName, ".go", "_mock.go", 1)

	file, err := os.Create(fileName)

	if err != nil {
		fmt.Println("Error creating file: ", err)
	}

	_, err = io.WriteString(file, toWrite)

	if err != nil {
		fmt.Println("Error writing file: ", err)
	}

	file.Close()
	runFmt(fileName)
}

func runFmt(fileName string) {

	path, err := exec.LookPath("gofmt")
	if err != nil {
		fmt.Println("Skipping... gofmt not present in Path:", path)
		return
	}

	cmd := exec.Command("gofmt", "-w", fileName)
	err = cmd.Start()
	if err != nil {
		fmt.Println("Error executing gofmt", err)
	}

	err = cmd.Wait()
	if err != nil {
		fmt.Println("Error executing go fmt on file", fileName, err)
	}
}

func stripSpecialCharsinPrefix(str string) string {

	specialChars := map[string]bool{
		"*": true,
		"[": true,
		"]": true,
		"(": true,
		")": true,
		" ": true,
	}

	if len(str) > 0 {

		flag := true
		for flag {

			_, exists := specialChars[string(str[0])]
			if exists {
				str = strings.Trim(str, string(str[0]))

			} else {
				flag = false
			}
		}
	}

	return str
}
