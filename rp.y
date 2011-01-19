/* nex rp.nex && goyacc rp.y && 6g rp.nn.go y.go && 6l rp.nn.6 */
%{
package main
import "fmt"
%}

%union { n int }

%token NUM
%%
input:    /* empty */
       | input line
;

line:     '\n'
       | exp '\n'      { println($1.n); }
;

exp:     NUM           { $$.n = $1.n;        }
       | exp exp '+'   { $$.n = $1.n + $2.n; }
       | exp exp '-'   { $$.n = $1.n - $2.n; }
       | exp exp '*'   { $$.n = $1.n * $2.n; }
       | exp exp '/'   { $$.n = $1.n / $2.n; }
	/* Unary minus    */
       | exp 'n'       { $$.n = -$1.n;       }
;
%%
