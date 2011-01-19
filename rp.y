/* s/yySymType/YYSymType/g and s/yyParse/YYParse/g on the generated parser, and
 * this will work with rp.nex
 */
%{
package rp
import "fmt"
%}

%union { N int }

%token NUM
%%
input:    /* empty */
       | input line
;

line:     '\n'
       | exp '\n'      { println($1.N); }
;

exp:     NUM           { $$.N = $1.N;        }
       | exp exp '+'   { $$.N = $1.N + $2.N; }
       | exp exp '-'   { $$.N = $1.N - $2.N; }
       | exp exp '*'   { $$.N = $1.N * $2.N; }
       | exp exp '/'   { $$.N = $1.N / $2.N; }
	/* Unary minus    */
       | exp 'n'       { $$.N = -$1.N;       }
;
%%
