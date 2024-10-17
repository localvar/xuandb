%{
package parser

import "net/netip"
import "time"

import "github.com/localvar/xuandb/pkg/query/ast"
%}

%union {
	stmt    ast.Statement
    expr    ast.Expr
    str     string
    int     uint64
    float   float64
    bool    bool
}

// Identifiers
// Refer comments of the 'init' function for orders of the tokens below.
%token<str>    IDENT

// Keywords
%token ALTER   CREATE   DELETE   DROP   SET   SELECT   SHOW   UPDATE
       USER   DATABASE   NODE   CLUSTER   VOTER   NONVOTER
       AS   AT   BY   FOR   IN   ON   WHERE   WITH
       GROUP   LIMIT   OFFSET   JOIN   BETWEEN   DURATION   PASSWORD

// comments
%token<str>    COMMENT

// Value tokens
%token<str>    VAL_STR
%token<int>    VAL_INT  VAL_DURATION
%token<float>  VAL_FLT
%token<bool>   VAL_BOOL

// Operators
%left  OP_ASSIGN
%left  OP_OR
%left  OP_XOR
%left  OP_AND
%right OP_NOT
%left  OP_EQU    OP_NOT_EQU    OP_GT    OP_GTE   OP_LT   OP_LTE
       OP_MATCH  OP_NOT_MATCH
%left  OP_BITWISE_OR
%left  OP_BITWISE_AND
%left  OP_LSHIFT OP_RSHIFT
%left  OP_ADD    OP_SUB
%left  OP_MUL    OP_DIV    OP_MOD
%left  OP_BITWISE_XOR
%right OP_SIGN   OP_BITWISE_NOT

// ERR_TOKEN represents an invalid token, to make test cases work, it must be
// the last token also.
%token<str> ERR_TOKEN

%type<str>  ADDR_PORT

// Statements
%type<stmt> STATEMENT
            CREATE_USER_STATEMENT SHOW_USER_STATEMENT DROP_USER_STATEMENT SET_PASSWORD_STATEMENT
            CREATE_DATABASE_STATEMENT DROP_DATABASE_STATEMENT SHOW_DATABASE_STATEMENT
            JOIN_NODE_STATEMENT DROP_NODE_STATEMENT SHOW_NODE_STATEMENT


%%

STATEMENT:
    CREATE_USER_STATEMENT
	{
        yylex.(*Lexer).Result = $1
		$$ = $1
	}
    | DROP_USER_STATEMENT
    {
        yylex.(*Lexer).Result = $1
		$$ = $1
    }
    | SET_PASSWORD_STATEMENT
    {
        yylex.(*Lexer).Result = $1
		$$ = $1
    }
    | SHOW_USER_STATEMENT
    {
        yylex.(*Lexer).Result = $1
        $$ = $1
    }
    | CREATE_DATABASE_STATEMENT
    {
        yylex.(*Lexer).Result = $1
        $$ = $1
    }
    | DROP_DATABASE_STATEMENT
    {
        yylex.(*Lexer).Result = $1
        $$ = $1
    }
    | SHOW_DATABASE_STATEMENT
    {
        yylex.(*Lexer).Result = $1
        $$ = $1
    }
    | JOIN_NODE_STATEMENT
    {
        yylex.(*Lexer).Result = $1
        $$ = $1
    }
    | DROP_NODE_STATEMENT
    {
        yylex.(*Lexer).Result = $1
        $$ = $1
    }
    | SHOW_NODE_STATEMENT
    {
        yylex.(*Lexer).Result = $1
        $$ = $1
    }

ADDR_PORT:
    VAL_STR
    {
        if _, err := netip.ParseAddrPort($1); err != nil {
            yylex.Error("invalid network address")
            goto ret1
        }
        $$ = $1
    }

CREATE_USER_STATEMENT:
    CREATE USER IDENT WITH PASSWORD VAL_STR
    {
        $$ = &ast.CreateUserStatement{Name: $3, Password: $6}
    }

DROP_USER_STATEMENT:
    DROP USER IDENT
    {
        $$ = &ast.DropUserStatement{Name: $3}
    }

SET_PASSWORD_STATEMENT:
    SET PASSWORD FOR IDENT OP_EQU VAL_STR
    {
        $$ = &ast.SetPasswordStatement{Name: $4, Password: $6}
    }

SHOW_USER_STATEMENT:
    SHOW USER
    {
        $$ = &ast.ShowUserStatement{}
    }

CREATE_DATABASE_STATEMENT:
    CREATE DATABASE IDENT WITH DURATION VAL_DURATION
    {
        $$ = &ast.CreateDatabaseStatement{Name: $3, Duration: time.Duration($6)}
    }

DROP_DATABASE_STATEMENT:
    DROP DATABASE IDENT
    {
        $$ = &ast.DropDatabaseStatement{Name: $3}
    }

SHOW_DATABASE_STATEMENT:
    SHOW DATABASE
    {
        $$ = &ast.ShowDatabaseStatement{}
    }

JOIN_NODE_STATEMENT:
    JOIN NODE IDENT AT ADDR_PORT AS VOTER
    {
        $$ = &ast.JoinNodeStatement{ID: $3, Addr: $5, Voter: true}
    }
    | JOIN NODE IDENT AT ADDR_PORT AS NONVOTER
    {
        $$ = &ast.JoinNodeStatement{ID: $3, Addr: $5, Voter: false}
    }

DROP_NODE_STATEMENT:
    DROP NODE IDENT
    {
        $$ = &ast.DropNodeStatement{ID: $3}
    }

SHOW_NODE_STATEMENT:
    SHOW NODE
    {
        $$ = &ast.ShowNodeStatement{}
    }
        
%%

// keywords maps of keyword strings to their corresponding IDs, keywords that
// cannot be defined as tokens are put into the map directly.
var keywords = map[string]int {
    "DIV": OP_DIV,
    "MOD": OP_MOD,
    "AND": OP_AND,
    "OR": OP_OR,
    "XOR": OP_XOR,
    "NOT": OP_NOT,
    "LIKE": OP_MATCH,
}

// init add other keywords to the keyword map, to make this possible, IDENT
// must be defined as the last token before the keyword tokens, and COMMENT
// must be defined as the first token after the keyword tokens.
func init() {
    id, idx := 0, 0

    // Find the index of the IDENT token.
    for {
        tok := yyToknames[idx]
        idx++
        if tok == "IDENT" {
            id = IDENT+1
            break
        }
    }

    // Put tokens before the value tokens to the keyword map.
    for idx < len(yyToknames) {
        tok := yyToknames[idx]
        if tok == "COMMENT" {
			break
		}
        keywords[tok] = id
        id++
        idx++
    }
}
