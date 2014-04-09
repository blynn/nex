#!/bin/bash
${NEXBIN:=../nex} tacky.nex
go tool yacc tacky.y
# Could use nex instead of ed, but that'd be a little gratuitous.
printf '/NEX_END_OF_LEXER_STRUCT/i\np *Tacky\n.\nw\nq\n' | ed -s tacky.nn.go
go build tacky.go tacky.nn.go y.go
