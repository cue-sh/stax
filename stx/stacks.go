package stx

import (
	"errors"
	"reflect"
	"regexp"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/token"
	"github.com/TangoGroup/stx/logger"
)

// Stack represents the decoded value of stacks[stackname]
type Stack struct {
	Name, Profile, Region, Environment, RegionCode string
	Overrides                                      map[string]struct {
		SopsProfile string
		Map         map[string]string
	}
	DependsOn   []string
	Tags        map[string]string
	TagsEnabled bool
}

// StacksIterator is a wrapper around cue.Iterator that allows for filtering based on stack fields
type StacksIterator struct {
	cueIter       cue.Iterator
	buildInstance *build.Instance
	flags         Flags
	log           *logger.Logger
}

// NewStacksIterator returns *StacksIterator
func NewStacksIterator(cueInstance *cue.Instance, buildInstance *build.Instance, flags Flags, log *logger.Logger) (*StacksIterator, error) {
	log.Debug("Getting stacks...")

	stacks := cueInstance.Value().Lookup("Stacks")
	if !stacks.Exists() {
		return nil, errors.New("Stacks is undefined")
	}

	fields, fieldsErr := stacks.Fields()
	if fieldsErr != nil {
		return nil, fieldsErr
	}

	return &StacksIterator{cueIter: fields, buildInstance: buildInstance, flags: flags, log: log}, nil
}

// Next moves the index forward and applies global filters. returns true if there is a value that passes the filters
func (it *StacksIterator) Next() bool {
	if !it.cueIter.Next() {
		return false
	}

	currentValue := it.cueIter.Value()

	if it.flags.StackNameRegexPattern != "" {
		stackName, _ := currentValue.Label()
		var stackNameRegexp *regexp.Regexp
		var stackNameRegexpErr error

		it.log.Debug("Compiling --stacks regexp...")
		stackNameRegexp, stackNameRegexpErr = regexp.Compile(it.flags.StackNameRegexPattern)
		if stackNameRegexpErr != nil {
			it.log.Fatal(stackNameRegexpErr)
		}
		if !stackNameRegexp.MatchString(stackName) {
			return it.Next()
		}
	}

	// apply filters to the current value
	if it.flags.Environment != "" {
		environmentValue := currentValue.Lookup("Environment")
		if !environmentValue.Exists() {
			return it.Next()
		}
		environment, environmentErr := environmentValue.String()
		if environmentErr != nil {
			it.log.Error(environmentErr)
			return it.Next()
		}
		if it.flags.Environment != environment {
			return it.Next()
		}
	}

	if it.flags.RegionCode != "" {
		regionCodeValue := currentValue.Lookup("RegionCode")
		if !regionCodeValue.Exists() {
			return it.Next()
		}
		regionCode, regionCodeErr := regionCodeValue.String()
		if regionCodeErr != nil {
			it.log.Error(regionCodeErr)
			return it.Next()
		}
		if it.flags.RegionCode != regionCode {
			return it.Next()
		}
	}

	if it.flags.Profile != "" {
		it.log.Debug("Evaluating --profile", it.flags.Profile)
		profileValue := currentValue.Lookup("Profile")
		if !profileValue.Exists() {
			return it.Next()
		}
		profile, profileErr := profileValue.String()
		if profileErr != nil {
			it.log.Error(profileErr)
			return it.Next()
		}
		if it.flags.Profile != profile {
			return it.Next()
		}
	}

	if it.flags.Has != "" {
		it.log.Debug("Evaluating --has", it.flags.Has)
		path := strings.Split(it.flags.Has, ".")
		hasValue := currentValue.Lookup(path...)
		if !hasValue.Exists() {
			return it.Next()
		}
	}

	if it.flags.Imports != "" {

		it.log.Debug("Compiling --imports regexp...")
		it.log.Infof("%s\n", it.flags.Imports)
		imports := strings.ReplaceAll(it.flags.Imports, " ", "")
		splits := strings.Split(imports, ":")
		importStr := splits[len(splits)-1]
		importSplits := strings.Split(importStr, ".")
		idx := strings.LastIndex(importStr, ".")

		var importPath, importField string
		if len(importSplits) > 1 {
			// importPath = importSplits[0]
			// importField = importSplits[1]
			importPath = importStr[0:idx]
			importField = importStr[idx+1:]
		} else {
			// importPath = importSplits[0]
			// importField = ""
			importPath = importStr[0:]
			importField = ""
		}
		it.log.Infof("importStr: %+v\n", importStr)
		it.log.Info("importPath", importPath)
		it.log.Info("importField", importField)
		var elems []string
		if len(splits) > 1 {
			elems = splits[0 : len(splits)-1]
		} else {
			elems = []string{"Stack"}
		}

		if elems[0] != "Stack" {
			it.log.Fatal("Invalid --imports formatting. Selector must be empty (for Stack) or begin with `Stack`\n")
		}

		it.log.Infof("elems: %+v\n", elems)

		node := currentValue
		for _, elem := range elems[1:] {
			fields, _ := currentValue.Fields()
			for fields.Next() {
				field := fields.Value()
				fieldLabel, _ := field.Label()
				if strings.TrimSpace(elem) == fieldLabel {
					node = field
					break
				}
			}
		}

		pkgName := ""

		for _, imp := range it.buildInstance.Imports {
			if imp.ImportPath == importPath {
				pkgName = imp.PkgName
			}
		}

		nodeSyntax := node.Syntax(cue.Raw())
		b, _ := format.Node(nodeSyntax)
		it.log.Infof("%s\n", b)

		depth := 0
		found := false
		// ast.Walk(a.Decls[1], func(node ast.Node) bool {
		ast.Walk(nodeSyntax, func(node ast.Node) bool {
			it.log.Infof("%d: %s%+v\n", depth, strings.Repeat("  ", depth), node)
			switch n := node.(type) {
			// Comments and fields
			case *ast.File:
				depth = depth + 1
				return true
			case *ast.SelectorExpr:
				it.log.Infof("%d: %s Found Selector: %+v\n", depth, strings.Repeat("  ", depth), n)
				// it.log.Infof("%d: %s Found Selector: %s.%s\n", depth, strings.Repeat("  ", depth), n.X, n.Sel.Name)
				it.log.Infof("%d: %s Found type: %s.\n", depth, strings.Repeat("  ", depth), reflect.TypeOf(n.X))
				id, isIdent := n.X.(*ast.Ident)
				if isIdent && id.Name == pkgName && n.Sel.Name == importField {
					found = true
				}

				return false
			case *ast.BinaryExpr:
				if n.Op == token.AND {
					depth = depth + 1
					return true
				}
				return false
			case *ast.ParenExpr, *ast.EmbedDecl:
				it.log.Infof("%d: %s%+v\n", depth, strings.Repeat("  ", depth), n)
				depth = depth + 1
				return true
			default:
				return false
			}
		}, func(ast.Node) {
			depth = depth - 1
			return
		})
		stackName, _ := currentValue.Label()
		if found {
			it.log.Infof("%s is an %s.%s\n", stackName, importPath, importField)
		} else {
			it.log.Errorf("%s is not an %s.%s\n", stackName, importPath, importField)
		}

		// importsRegexp, importsRegexpErr := regexp.Compile(it.flags.Imports)
		// if importsRegexpErr != nil {
		// 	log.Fatal(importsRegexpErr)
		// }
		// raw := currentValue.Syntax(cue.Raw())
		// b, _ := format.Node(raw)
		// it.log.Infof("%s\n", b)

		for _, imp := range it.buildInstance.Imports {
			if imp.ImportPath == "github.com/TangoGroup/sre/Resources/RDS/Legacy" {
				// cueInst := cue.Build([]*build.Instance{imp})[0]
				// ac := cueInst.Value().Lookup("AuroraCluster")
				// it.log.Infof("%+v\n", imp.PkgName)
				// it.log.Infof("import: %+v\n", imp)
				// it.log.Infof("currentValue: %+v\n", currentValue)
				// stackStruct, _ := currentValue.Struct(cue.Raw())

				// fi, _ := stackStruct.FieldByName("Template")1

				// it.log.Infof("%+v\n", fi)

				// raw := currentValue.Syntax(cue.Raw())
				// b, _ := format.Node(raw)
				// it.log.Infof("%s\n", b)
				fields, _ := currentValue.Fields()
				for fields.Next() {
					field := fields.Value()
					s, _ := field.Label()
					if s == "Template" {
						// it.log.Info(s)
						conc := field.Syntax(cue.Raw()).(*ast.File)
						// it.log.Infof("conc: Found type: %s.\n", reflect.TypeOf(conc))
						b, _ := format.Node(conc)
						// bs := string(b)
						it.log.Infof("%s\n", b)
						// a, _ := parser.ParseFile("tmp", bs)
						depth := 0
						found := false
						// ast.Walk(a.Decls[1], func(node ast.Node) bool {
						ast.Walk(conc.Decls[1], func(node ast.Node) bool {
							// it.log.Infof("%d: %s%+v\n", depth, strings.Repeat("  ", depth), node)
							switch n := node.(type) {
							// Comments and fields
							case *ast.SelectorExpr:
								// it.log.Infof("%d: %s Found Selector: %+v\n", depth, strings.Repeat("  ", depth), n)
								// it.log.Infof("%d: %s Found Selector: %s.%s\n", depth, strings.Repeat("  ", depth), n.X, n.Sel.Name)
								// it.log.Infof("%d: %s Found type: %s.\n", depth, strings.Repeat("  ", depth), reflect.TypeOf(n.X))
								id, _ := n.X.(*ast.Ident)
								if id.Name == imp.PkgName && n.Sel.Name == "AuroraCluster" {
									found = true
								}
								return false
							case *ast.BinaryExpr:
								if n.Op == token.AND {
									depth = depth + 1
									return true
								}
								return false
							case *ast.ParenExpr, *ast.EmbedDecl:
								// it.log.Infof("%d: %s%+v\n", depth, strings.Repeat("  ", depth), n)
								depth = depth + 1
								return true
							default:
								return false
							}
						}, func(ast.Node) {
							depth = depth - 1
							return
						})
						stackName, _ := currentValue.Label()
						if found {
							it.log.Infof("%s is an %s.AuroraCluster\n", stackName, imp.ImportPath)
						} else {
							it.log.Errorf("%s is not an %s.AuroraCluster\n", stackName, imp.ImportPath)
						}
						// it.log.Infof("%+v\n", a.Decls[1].(*ast.EmbedDecl).Expr.(*ast.BinaryExpr).X)
						// for _, el := range conc.Elts {
						// 	el.
						// }
						// op, expr := field.Expr()
						// it.log.Infof("op: %+v\n", op)
						// for _, val := range expr {
						// it.log.Infof("val: %+v\n", val)
						// stackName, _ := currentValue.Label()
						// if ac.Equals(val) {
						// 	it.log.Errorf("%s is not an %s.AuroraCluster\n", stackName, imp.ImportPath)
						// } else {
						// 	it.log.Infof("%s is an %s.AuroraCluster\n", stackName, imp.ImportPath)
						// }
						// }
					}
				}

				// it.log.Infof("%+v\n", ac)
				// unified :=
				// err := unified.Validate(cue.Concrete(true))
				// // stackName, _ := currentValue.Label()
				// val := currentValue.Lookup("Template")
				// if ac.Subsumes(val) {
				// 	it.log.Errorf("%s is not an %s.AuroraCluster\n", stackName, imp.ImportPath)
				// } else {
				// 	it.log.Infof("%s is an %s.AuroraCluster\n", stackName, imp.ImportPath)
				// }
			}
		}
	}

	return true
}

// Value returns the value from the cue.Iterator
func (it *StacksIterator) Value() cue.Value {
	// _, v2 := it.cueIter.Value().Eval().Expr()
	// it.log.Infof("%+v\n", v2)
	// template := v2[0].Lookup("Template")
	// refs := template.References()
	// it.log.Infof("%+v\n", refs)
	// it.log.Infof("%+v\n", it.cueIter.Value().Lookup("Template").References())
	// op, vals := it.cueIter.Value().Lookup("Template").Expr()
	// it.log.Infof("op: %+v\n", op)
	// for _, val := range vals {
	// 	_, vpaths := val.Reference()
	// 	it.log.Infof("  x%+v\n", vpaths)
	// }
	// inst, paths := v2[0].Lookup("Template").Reference()
	// it.log.Infof("%+v\n%+v\n", inst.Dir, paths)
	return it.cueIter.Value()
}
