package main

import (
	"go/token"
	"strconv"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/pijng/goinject"
)

type autoinstrumentModifier struct{}

func (mm autoinstrumentModifier) Modify(f *dst.File, dec *decorator.Decorator, res *decorator.Restorer) *dst.File {
	for _, decl := range f.Decls {
		funcDecl, isFunc := decl.(*dst.FuncDecl)
		if !isFunc {
			continue
		}

		var contextArgName string
		for _, param := range funcDecl.Type.Params.List {
			paramIdent, isIdent := param.Type.(*dst.Ident)
			if !isIdent {
				continue
			}

			if paramIdent.Path == "context" && paramIdent.Name == "Context" && param.Names[0].Name != "_" {
				contextArgName = param.Names[0].Name
				break
			}
		}

		for _, param := range funcDecl.Type.Params.List {
			starExpr, isPointer := param.Type.(*dst.StarExpr)
			if !isPointer {
				continue
			}
			paramIdent, isIdent := starExpr.X.(*dst.Ident)
			if !isIdent {
				continue
			}

			if paramIdent.Path == "net/http" && paramIdent.Name == "Request" && param.Names[0].Name != "_" {
				contextArgName = param.Names[0].Name + ".Context()"
			}
		}

		funcName := f.Name.Name + "." + funcDecl.Name.Name

		spanStmt := buildSpan(funcName, contextArgName)
		funcDecl.Body.List = append(spanStmt.List, funcDecl.Body.List...)
	}

	return f
}

func main() {
	goinject.Process(autoinstrumentModifier{})
}

// span := StartSpan("main")
// defer() {
//   span.End()
// }()

func buildSpan(funcName string, contextArgName string) dst.BlockStmt {
	spanFunc := "StartSpan"
	if contextArgName != "" {
		spanFunc = "StartSpanCtx"
	}

	args := make([]dst.Expr, 0)
	if contextArgName != "" {
		args = append(args, &dst.Ident{Name: contextArgName})
	}
	args = append(args, &dst.BasicLit{Kind: token.STRING, Value: strconv.Quote(funcName)})

	return dst.BlockStmt{
		List: []dst.Stmt{
			// gomSpan := gootelinstrument.StartSpan(funcName)
			&dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{Name: "gomSpan"},
				},
				Tok: token.DEFINE, // :=
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun:  &dst.Ident{Path: "github.com/pijng/gootelinstrument", Name: spanFunc},
						Args: args,
					},
				},
			},
			// defer func() { gomSpan.End() }()
			&dst.DeferStmt{
				Call: &dst.CallExpr{
					Fun: &dst.FuncLit{
						Type: &dst.FuncType{
							Params: &dst.FieldList{},
						},
						Body: &dst.BlockStmt{
							List: []dst.Stmt{
								&dst.ExprStmt{
									X: &dst.CallExpr{
										Fun: &dst.SelectorExpr{
											X:   &dst.Ident{Name: "gomSpan"},
											Sel: &dst.Ident{Name: "End"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}