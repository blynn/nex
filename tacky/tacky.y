%{
package main
import "fmt"
%}

%union {
  s string
  expr *Expr
}

%token DEF_FORM
%token ASSIGN
%token ID
%token MONEY
%token FRAC
%token XREF
%token FUNC
%left '+' '-'
%left '*' '/'
%%
input: /* empty */
       | input stuff

stuff: DEF_FORM { cast(yylex).BeginForm($1.s) }
     | '}'      { cast(yylex).EndForm() }
     | ASSIGN   { cast(yylex).Assign($1.s) }
     | expr     { cast(yylex).Expr($1.expr) }

expr: atom
    | expr '+' expr { $$.expr = NewOp($2.s, $1.expr, $3.expr) }
    | expr '-' expr { $$.expr = NewOp($2.s, $1.expr, $3.expr) }
    | expr '*' expr { $$.expr = NewOp($2.s, $1.expr, $3.expr) }
    | expr '/' expr { $$.expr = NewOp($2.s, $1.expr, $3.expr) }

atom: MONEY        { $$.expr = NewExpr("$", $1.s) }
    | ID           { $$.expr = NewExpr("ID", $1.s) }
    | '(' expr ')' { $$.expr = $2.expr }
    | XREF         { $$.expr = NewExpr("XREF", $1.s) }
    | FUNC arglist ')' { $$.expr = NewFun($1.s, $2.expr) }
    | FRAC         { $$.expr = NewExpr("%", $1.s) }

arglist: expr { $$.expr = NewExpr("", ""); $$.expr.AddKid($1.expr) }
       | arglist ',' expr { $1.expr.AddKid($3.expr) }

%%
func cast(y yyLexer) *Tacky { return y.(*Lexer).p }
