package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Variable struct {
	Type  string
	Value string
}

func evaluateMath(expr string) (string, error) {
	expr = strings.ReplaceAll(expr, " ", "")
	var op rune
	opIdx := -1
	for i, r := range expr {
		if r == '+' || r == '-' || r == '*' || r == '/' {
			op = r
			opIdx = i
			break
		}
	}
	if opIdx == -1 {
		return expr, nil
	}
	leftStr := expr[:opIdx]
	rightStr := expr[opIdx+1:]

	leftFl, err1 := strconv.ParseFloat(leftStr, 64)
	rightFl, err2 := strconv.ParseFloat(rightStr, 64)
	if err1 != nil || err2 != nil {
		return leftStr + string(op) + rightStr, nil
	}

	var result float64
	switch op {
	case '+':
		result = leftFl + rightFl
	case '-':
		result = leftFl - rightFl
	case '*':
		result = leftFl * rightFl
	case '/':
		if rightFl == 0 {
			return "", fmt.Errorf("division by zero")
		}
		result = leftFl / rightFl
	}
	if float64(int64(result)) == result {
		return strconv.FormatInt(int64(result), 10), nil
	}
	return strconv.FormatFloat(result, 'f', -1, 64), nil
}

func checkCondition(left, op, right string) bool {
	if op == "==" {
		return left == right
	}
	if op == "!=" {
		return left != right
	}
	lNum, err1 := strconv.ParseFloat(left, 64)
	rNum, err2 := strconv.ParseFloat(right, 64)
	if err1 == nil && err2 == nil {
		switch op {
		case ">":
			return lNum > rNum
		case "<":
			return lNum < rNum
		case ">=":
			return lNum >= rNum
		case "<=":
			return lNum <= rNum
		}
	}
	return false
}

func debugWordCode(docText string, r *http.Request, rootDir string) string {
	docText = strings.ReplaceAll(docText, "\r\n", "\n")
	lines := strings.Split(docText, "\n")

	var htmlOutput strings.Builder
	isReadingCode := false
	textColor := "black"
	memory := make(map[string]Variable)

	type IfState struct {
		IsActive       bool
		ConditionMet   bool
		ElseBlockReady bool
	}
	var ifStack []IfState

	type WhileState struct {
		LineIndex int
		Condition string
	}
	var whileStack []WhileState

	const redColor = "\033[31m"
	const resetColor = "\033[0m"

	issetRegex := regexp.MustCompile(`\?isset:([a-zA-Z0-9_-]+)`)

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		currentLine := strings.TrimSpace(line)
		currentLine = strings.ReplaceAll(currentLine, "![SPACE]", " ")

		if currentLine == "<?wsp" {
			isReadingCode = true
			continue
		}

		if currentLine == "!>" {
			isReadingCode = false
			continue
		}

		if r != nil {
			queryParams := r.URL.Query()
			r.ParseForm()
			postParams := r.PostForm

			matches := issetRegex.FindAllStringSubmatch(currentLine, -1)
			for _, match := range matches {
				if len(match) > 1 {
					paramName := match[1]
					placeholder := "?isset:" + paramName
					_, inGet := queryParams[paramName]
					_, inPost := postParams[paramName]
					if inGet || inPost {
						currentLine = strings.ReplaceAll(currentLine, placeholder, "true")
						line = strings.ReplaceAll(line, placeholder, "true")
					} else {
						currentLine = strings.ReplaceAll(currentLine, placeholder, "false")
						line = strings.ReplaceAll(line, placeholder, "false")
					}
				}
			}

			for key, values := range queryParams {
				if len(values) > 0 {
					placeholder := "?args:" + key
					line = strings.ReplaceAll(line, placeholder, values[0])
					currentLine = strings.ReplaceAll(currentLine, placeholder, values[0])
				}
			}

			var keys []string
			for k := range queryParams {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for idx, key := range keys {
				values := queryParams[key]
				if len(values) > 0 {
					placeholder := "?args:" + strconv.Itoa(idx)
					line = strings.ReplaceAll(line, placeholder, values[0])
					currentLine = strings.ReplaceAll(currentLine, placeholder, values[0])
				}
			}

			for key, values := range postParams {
				if len(values) > 0 {
					placeholder := "?post:" + key
					line = strings.ReplaceAll(line, placeholder, values[0])
					currentLine = strings.ReplaceAll(currentLine, placeholder, values[0])
				}
			}
		} else {
			matches := issetRegex.FindAllStringSubmatch(currentLine, -1)
			for _, match := range matches {
				if len(match) > 1 {
					placeholder := "?isset:" + match[1]
					currentLine = strings.ReplaceAll(currentLine, placeholder, "false")
					line = strings.ReplaceAll(line, placeholder, "false")
				}
			}
		}

		if !isReadingCode {
			skipHtml := false
			for _, state := range ifStack {
				if !state.IsActive {
					skipHtml = true
					break
				}
			}
			if len(whileStack) > 0 {
				cond := whileStack[len(whileStack)-1].Condition
				for varName, varData := range memory {
					cond = strings.ReplaceAll(cond, "?mem:"+varName, varData.Value)
				}
				wArgs := strings.Split(cond, " ")
				if len(wArgs) >= 3 && !checkCondition(wArgs[0], wArgs[1], wArgs[2]) {
					skipHtml = true
				}
			}
			if skipHtml {
				continue
			}

			for varName, varData := range memory {
				placeholder := "?mem:" + varName
				line = strings.ReplaceAll(line, placeholder, varData.Value)
			}
			htmlOutput.WriteString(line + "\n")
			continue
		}

		if currentLine == "" || strings.HasPrefix(currentLine, ";;") {
			continue
		}

		var command, arguments string
		spacePos := strings.Index(currentLine, " ")

		if spacePos > 0 {
			command = strings.ToUpper(currentLine[:spacePos])
			arguments = currentLine[spacePos+1:]
		} else {
			command = strings.ToUpper(currentLine)
			arguments = ""
		}

		if command != "$MEM" && command != "INCLUDE" && command != "WHILE" {
			for varName, varData := range memory {
				placeholder := "?mem:" + varName
				arguments = strings.ReplaceAll(arguments, placeholder, varData.Value)
			}
		}

		if command == "IF" || command == "ELSEIF" || command == "ELSE" || command == "ENDIF" {
			switch command {
			case "IF":
				ifArgs := strings.Split(arguments, " ")
				if len(ifArgs) >= 3 {
					left := ifArgs[0]
					op := ifArgs[1]
					right := ifArgs[2]
					parentActive := true
					for _, state := range ifStack {
						if !state.IsActive {
							parentActive = false
							break
						}
					}
					isTrue := parentActive && checkCondition(left, op, right)
					ifStack = append(ifStack, IfState{IsActive: isTrue, ConditionMet: isTrue, ElseBlockReady: true})
				} else {
					fmt.Fprintln(os.Stderr, redColor+"(Error) IF requires 3 arguments!"+resetColor)
				}
			case "ELSEIF":
				if len(ifStack) > 0 {
					idx := len(ifStack) - 1
					if ifStack[idx].ElseBlockReady {
						if ifStack[idx].ConditionMet {
							ifStack[idx].IsActive = false
						} else {
							ifArgs := strings.Split(arguments, " ")
							if len(ifArgs) >= 3 {
								left := ifArgs[0]
								op := ifArgs[1]
								right := ifArgs[2]
								parentActive := true
								for i := 0; i < idx; i++ {
									if !ifStack[i].IsActive {
										parentActive = false
										break
									}
								}
								isTrue := parentActive && checkCondition(left, op, right)
								ifStack[idx].IsActive = isTrue
								if isTrue {
									ifStack[idx].ConditionMet = true
								}
							} else {
								fmt.Fprintln(os.Stderr, redColor+"(Error) ELSEIF requires 3 arguments!"+resetColor)
							}
						}
					} else {
						fmt.Fprintln(os.Stderr, redColor+"(Error) Unexpected ELSEIF!"+resetColor)
					}
				} else {
					fmt.Fprintln(os.Stderr, redColor+"(Error) ELSEIF without IF!"+resetColor)
				}
			case "ELSE":
				if len(ifStack) > 0 {
					idx := len(ifStack) - 1
					if ifStack[idx].ElseBlockReady {
						ifStack[idx].IsActive = !ifStack[idx].ConditionMet
						ifStack[idx].ElseBlockReady = false
					} else {
						fmt.Fprintln(os.Stderr, redColor+"(Error) Unexpected ELSE!"+resetColor)
					}
				} else {
					fmt.Fprintln(os.Stderr, redColor+"(Error) ELSE without IF!"+resetColor)
				}
			case "ENDIF":
				if len(ifStack) > 0 {
					ifStack = ifStack[:len(ifStack)-1]
				} else {
					fmt.Fprintln(os.Stderr, redColor+"(Error) ENDIF without IF!"+resetColor)
				}
			}
			continue
		}

		skipLine := false
		for _, state := range ifStack {
			if !state.IsActive {
				skipLine = true
				break
			}
		}
		if skipLine {
			continue
		}

		if command == "WHILE" {
			whileStack = append(whileStack, WhileState{LineIndex: i, Condition: arguments})
			evalCond := arguments
			for varName, varData := range memory {
				evalCond = strings.ReplaceAll(evalCond, "?mem:"+varName, varData.Value)
			}
			wArgs := strings.Split(evalCond, " ")
			if len(wArgs) >= 3 {
				if !checkCondition(wArgs[0], wArgs[1], wArgs[2]) {
					depth := 1
					for k := i + 1; k < len(lines); k++ {
						tLine := strings.TrimSpace(lines[k])
						if strings.HasPrefix(strings.ToUpper(tLine), "WHILE") {
							depth++
						} else if strings.ToUpper(tLine) == "ENDWHILE" {
							depth--
							if depth == 0 {
								i = k
								whileStack = whileStack[:len(whileStack)-1]
								break
							}
						}
					}
				}
			}
			continue
		}

		if command == "ENDWHILE" {
			if len(whileStack) > 0 {
				top := whileStack[len(whileStack)-1]
				evalCond := top.Condition
				for varName, varData := range memory {
					evalCond = strings.ReplaceAll(evalCond, "?mem:"+varName, varData.Value)
				}
				wArgs := strings.Split(evalCond, " ")
				if len(wArgs) >= 3 && checkCondition(wArgs[0], wArgs[1], wArgs[2]) {
					i = top.LineIndex
				} else {
					whileStack = whileStack[:len(whileStack)-1]
				}
			} else {
				fmt.Fprintln(os.Stderr, redColor+"(Error) ENDWHILE without WHILE!"+resetColor)
			}
			continue
		}

		if command == "$QUIT" || command == "QUIT" {
			break
		}

		switch command {
		case "INCLUDE":
			for varName, varData := range memory {
				placeholder := "?mem:" + varName
				arguments = strings.ReplaceAll(arguments, placeholder, varData.Value)
			}
			incPath := filepath.Join(rootDir, arguments)
			incContent, err := os.ReadFile(incPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, redColor+"(Error) INCLUDE failed to read file: "+err.Error()+resetColor)
				continue
			}
			incResult := debugWordCode(string(incContent), r, rootDir)
			htmlOutput.WriteString(incResult)

		case "$MEM":
			memArgs := strings.Split(arguments, " ")
			if len(memArgs) >= 2 && (memArgs[0] == "++" || memArgs[0] == "--") {
				action := memArgs[0]
				name := memArgs[1]
				existing, exists := memory[name]
				if exists && (existing.Type == "integer" || existing.Type == "float") {
					valFl, _ := strconv.ParseFloat(existing.Value, 64)
					if action == "++" {
						valFl++
					} else {
						valFl--
					}
					var finalStr string
					if existing.Type == "integer" {
						finalStr = strconv.FormatInt(int64(valFl), 10)
					} else {
						finalStr = strconv.FormatFloat(valFl, 'f', -1, 64)
					}
					memory[name] = Variable{Type: existing.Type, Value: finalStr}
				} else {
					fmt.Fprintln(os.Stderr, redColor+"(Error) Increment/Decrement target error!"+resetColor)
				}
				continue
			}

			memArgs = strings.SplitN(arguments, " ", 4)
			if len(memArgs) >= 4 {
				action := strings.ToLower(memArgs[0])
				varType := strings.ToLower(memArgs[1])
				name := memArgs[2]
				value := memArgs[3]

				for varName, varData := range memory {
					placeholder := "?mem:" + varName
					value = strings.ReplaceAll(value, placeholder, varData.Value)
				}

				mathValue, err := evaluateMath(value)
				if err != nil {
					fmt.Fprintln(os.Stderr, redColor+"(Error) Math exception: "+err.Error()+resetColor)
					continue
				}
				value = mathValue

				if action == "add" {
					if varType == "string" || varType == "integer" || varType == "float" || varType == "boolean" {
						memory[name] = Variable{Type: varType, Value: value}
					} else {
						fmt.Fprintln(os.Stderr, redColor+"(Error) Invalid variable type for creation!"+resetColor)
					}
				} else if action == "edit" {
					existing, exists := memory[name]
					if exists {
						finalType := varType
						if varType == "?this" {
							finalType = existing.Type
						}
						finalValue := value
						if value == "?this" {
							finalValue = existing.Value
						}

						if finalType == "string" || finalType == "integer" || finalType == "float" || finalType == "boolean" {
							memory[name] = Variable{Type: finalType, Value: finalValue}
						} else {
							fmt.Fprintln(os.Stderr, redColor+"(Error) Invalid variable type for modification!"+resetColor)
						}
					} else {
						fmt.Fprintln(os.Stderr, redColor+"(Error) Variable '"+name+"' to edit does not exist!"+resetColor)
					}
				} else {
					fmt.Fprintln(os.Stderr, redColor+"(Error) Invalid memory action!"+resetColor)
				}
			} else {
				fmt.Fprintln(os.Stderr, redColor+"(Error) $mem requires more arguments!"+resetColor)
			}
		case "$P", "PARAGRAPH":
			htmlOutput.WriteString(fmt.Sprintf("<p style='color:%s;'>%s</p>\n", textColor, arguments))

		case "$STR", "STRING":
			htmlOutput.WriteString(fmt.Sprintf("<span style='color:%s;'>%s</span>", textColor, arguments))

		case "$D", "DATE":
			currentTime := time.Now().Format("02.01.2006")
			htmlOutput.WriteString(currentTime)

		case "$BR", "NEXT":
			htmlOutput.WriteString("<br>\n")

		case "$LOG", "LOG":
			currentTime := time.Now().Format("15:04:05")
			result := fmt.Sprintf("(Log at %s) %s", currentTime, arguments)
			fmt.Println(result)

		case "$PRINT", "PRINT":
			fmt.Print(arguments)

		case "$PRINTLN", "PRINTLN":
			fmt.Println(arguments)

		case "$PAR", "PARAM":
			paramArgs := strings.Split(arguments, " ")
			if len(paramArgs) >= 3 {
				obj := strings.ToUpper(paramArgs[0])
				prop := strings.ToUpper(paramArgs[1])
				val := strings.ToUpper(paramArgs[2])

				if obj == "TEXT" {
					if prop == "COLOR" {
						switch val {
						case "DEFAULT":
							textColor = "black"
						case "RED":
							textColor = "red"
						case "BLUE":
							textColor = "blue"
						case "GREEN":
							textColor = "green"
						case "YELLOW":
							textColor = "yellow"
						default:
							fmt.Fprintln(os.Stderr, redColor+"(Error) Invalid parameter value!"+resetColor)
						}
					} else {
						fmt.Fprintln(os.Stderr, redColor+"(Error) Unknown parameter property!"+resetColor)
					}
				} else {
					fmt.Fprintln(os.Stderr, redColor+"(Error) Unknown parameter object!"+resetColor)
				}
			} else {
				fmt.Fprintln(os.Stderr, redColor+"(Error) $par requires 3 arguments!"+resetColor)
			}

		default:
			fmt.Fprintln(os.Stderr, redColor+"(Error) Unknown instruction!"+resetColor)
		}
	}

	return htmlOutput.String()
}

func main() {
	if len(os.Args) > 2 {
		rootDir := os.Args[1]
		port := os.Args[2]

		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			urlPath := r.URL.Path
			if urlPath == "/" {
				urlPath = "/index.wsp"
			}

			targetFile := filepath.Join(rootDir, filepath.Clean(urlPath))

			content, err := os.ReadFile(targetFile)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				fmt.Fprintf(w, "<h1>404 Not Found</h1><p>The file %s could not be found.</p>", urlPath)
				return
			}

			htmlResult := debugWordCode(string(content), r, rootDir)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, htmlResult)
		})

		fmt.Printf("Starting WSP routing web server for folder '%s' on http://localhost:%s\n", rootDir, port)
		err := http.ListenAndServe(":"+port, nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, "\033[31m(Error) Server failed: "+err.Error()+"\033[0m")
		}
		return
	}

	if len(os.Args) == 2 {
		filePath := os.Args[1]
		content, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "\033[31m(Error) Could not read file: "+err.Error()+"\033[0m")
			os.Exit(1)
		}
		currentDir := filepath.Dir(filePath)
		result := debugWordCode(string(content), nil, currentDir)
		fmt.Print(result)
		return
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			return
		}

		TestCode := `
		<h1>File not loaded?</h1>
		`
		htmlResult := debugWordCode(TestCode, r, ".")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, htmlResult)
	})

	fmt.Println("No file specified. Starting default code on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
