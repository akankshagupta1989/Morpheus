package mock

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type FileInfo struct {
	parentDir   string
	lines       []string
	fset        *token.FileSet
	fileContent *ast.File
	filePath    string
}

func NewFileInfo(parent string) *FileInfo {

	return &FileInfo{
		parentDir: parent,
	}
}

func (f *FileInfo) FileParser(filePath string) {

	fset := token.NewFileSet()

	fileContent, err := parser.ParseFile(fset, filePath, nil, 0)

	if err != nil {
		fmt.Println("Error in parsing file", err)
		return
	}

	lines, err := readLines(filePath)

	if err != nil {
		fmt.Println("Error in parsing file line by line", err)
		return
	}

	f.fileContent = fileContent
	f.filePath = filePath
	f.fset = fset
	f.lines = lines
	/*
		ast.Inspect(fileContent, func(n ast.Node) bool {
			var s string
			fmt.Println("nodetype", reflect.TypeOf(n))
			switch x := n.(type) {

			case *ast.FuncDecl:
				fmt.Printf("%+v\n", x.Name)
				fmt.Println("isexported", x.Name.IsExported())

			case *ast.BasicLit:
				s = x.Value

			case *ast.Ident:
				s = x.Name

			}
			if s != "" {
				fmt.Printf("%s:\t%s\n", fset.Position(n.Pos()), s)
			}

			return true
		})
	*/
}

func (f *FileInfo) GetExportedFunctions(exportedFuncMap map[string]int) {

	ast.Inspect(f.fileContent, func(n ast.Node) bool {

		switch funcType := n.(type) {

		case *ast.FuncDecl:
			if funcType.Name.IsExported() {
				exportedFuncMap[funcType.Name.Name] = 0
			}
		}

		return true
	})
}

func parsePackageFiles(dirPath string) []FileInfo {

	files, _ := ioutil.ReadDir(dirPath)

	fileInfoArr := make([]FileInfo, 0)

	for _, f := range files {

		fileName := f.Name()

		if strings.HasSuffix(fileName, ".go") && !strings.HasSuffix(fileName, "_test.go") && !strings.HasSuffix(fileName, "_dev.go") {
			dirName := filepath.Base(dirPath)
			fmt.Println("dirname", dirName)
			fR := NewFileInfo(dirName)
			fullFilePath := dirPath + fileName
			fR.FileParser(fullFilePath)
			fileInfoArr = append(fileInfoArr, *fR)
		}
	}

	return fileInfoArr
}

func PreMockChecking(inputJsonPath, packagePath string) []FileInfo {

	exportedFuncMap := make(map[string]int, 0)
	fileInfoArr := parsePackageFiles(packagePath)

	for _, x := range fileInfoArr {
		x.GetExportedFunctions(exportedFuncMap)
	}
	fmt.Println(inputJsonPath)
	/*
		mockedDataMap := ParseJson(inputJsonPath)
		for funcKey, _ = range mockedDataMap {
			exportedFuncMap[funcKey] = 1
		}

		allExportedExists := true
		for funcName, value := range exportedFuncMap {
			if value == 0 {
				fmt.Println("Exported function ", funcName, " not present in input json")
				allExportedExists = false
			}
		}

		if !allExportedExists {
			fmt.Println("Please enter all exported functions in input json before mock build")
			os.Exit(1)
		}
	*/
	return fileInfoArr

}

func GenerateCodeForParsingJson(jsonPath, dirPath string) {

	packageName := filepath.Base(dirPath)
	tagName := fmt.Sprintf("// +build %smock\n", packageName)
	toWrite := fmt.Sprintf("%s\npackage%s\nimport (\n\"fmt\"\n)\n", tagName, packageName)
	toWrite := fmt.Sprintf("%svar ServicesMap = map[string]interface{}\n", toWrite)
	toWrite = fmt.Sprintf("%sfunc init() {\nfile, err := ioutil.ReadFile(%s)\n", toWrite, jsonPath)
	toWrite = fmt.Sprintf("%sif err!= nil {\nfmt.Println(\"func init: Error reading input json file\",err)\nos.Exit(1)\n}\n", toWrite)
	toWrite = fmt.Sprintf("%serr = json.Unmarshal([]byte(file), &servicesMap)\n", toWrite)
	toWrite = fmt.Sprintf("%sif err != nil {\nfmt.Println(\"func init: Unmarshalling error, input json not in correct format\",err)\n os.Exit(1)\n}\n", toWrite)
	toWrite = fmt.Sprintf("%s}", toWrite)

	file, err := os.Create(fmt.Sprintf("%s/mock_init.go", filePath.Dir(dirPath)))

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

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func ParseJson(inputJsonPath string) map[string]([]interface{}) {

	file, err := ioutil.ReadFile(inputJsonPath)
	if err != nil {
		fmt.Println("func ParseJson: Error reading input json file", err)
		os.Exit(1)
	}

	servicesMap := make(map[string]([]interface{}))

	err = json.Unmarshal([]byte(file), &servicesMap)
	if err != nil {
		fmt.Println("func ParseJson: Unmarshalling error, input json not in correct format", err)
		os.Exit(1)
	}
	return servicesMap
}

/*
type InterfaceData struct {
	interfaceName string
	interfaceType *ast.InterfaceType
	file          *FileInfo
}

func (f *FileInfo) SearchInterface(name string) (*InterfaceData, error) {

	for _, decl := range f.fileContent.Decls {
		fmt.Println("reflect", reflect.TypeOf(decl))
		gen, ok := decl.(*ast.GenDecl)
		fmt.Println("ok1", ok)
		if ok {
			for _, spec := range gen.Specs {
				fmt.Printf("spec%+v\n", reflect.TypeOf(spec))
				typespec, ok := spec.(*ast.TypeSpec)
				fmt.Println("ok2", ok)
				if ok {
					fmt.Printf("%+v\n", typespec.Type)
					if typespec.Name.Name == name {
						iface, ok := typespec.Type.(*ast.InterfaceType)
						if ok {
							fmt.Println("iface methods")
							fmt.Printf("%+v\n", iface.Methods.List[1])
							x := InterfaceData{name, iface, &FileInfo{f.fileContent, f.filePath}}
							return &x, nil

						} else {
							err := errors.New("interface not found")
							return nil, err
						}
					}
				}
			}
		}
	}
	return nil, nil
}

func parseFiles(dirPath string) []InterfaceData {

	fullDirPath, _ := filepath.Abs(dirPath)
	files, _ := ioutil.ReadDir(fullDirPath)

	interfaceArray := make([]InterfaceData, 0)

	for _, f := range files {

		fileName := f.Name()

		if strings.HasSuffix(fileName, ".go") {

			fR := NewFileInfo()
			fullFilePath, _ := filepath.Abs(fileName)
			fR.FileParser(fullFilePath)

			interfaceName := strings.Replace(fileName, ".go", "", -1)

			interfaceData, err := fR.SearchInterface(interfaceName)

			if err != nil {
				fmt.Println("Error:", err, " in file: ", fileName)
				continue
			}

			interfaceArray = append(interfaceArray, *interfaceData)
			GenerateMockFiles(interfaceArray)
		}
	}
	return interfaceArray
}

func GenerateMockFiles(interfaceArray []InterfaceData) {

	for i, _ := range interfaceArray {
		err := CreateMockFile(interfaceArray[i])

		if err != nil {
			fmt.Println("error in generating mock file", err)
		}
	}
}
*/
