package qlang

import (
	"qlang.io/exec.v2"
	"qlang.io/qlang.spec.v1"
	"qiniupkg.com/text/tpl.v1/interpreter.util"
)

// -----------------------------------------------------------------------------

const Grammar = `

term1 = factor *('*' factor/mul | '/' factor/quo | '%' factor/mod)

term2 = term1 *('+' term1/add | '-' term1/sub)

term3 = term2 *('<' term2/lt | '>' term2/gt | "==" term2/eq | "<=" term2/le | ">=" term2/ge | "!=" term2/ne)

term4 = term3 *("&&"/_mute term3/_code/_unmute/and)

expr = term4 *("||"/_mute term4/_code/_unmute/or)

s = (
	(IDENT '='! expr)/assign |
	(IDENT ','!)/name IDENT/name % ','/ARITY '=' expr /massign |
	(IDENT "++")/inc |
	(IDENT "--")/dec |
	(IDENT "+="! expr)/adda |
	(IDENT "-="! expr)/suba |
	(IDENT "*="! expr)/mula |
	(IDENT "/="! expr)/quoa |
	(IDENT "%="! expr)/moda |
	"return"! ?expr/ARITY /return |
	"include"! STRING/include |
	"defer"/_mute! expr/_code/_unmute/defer |
	expr)/xline

doc = s *(';'/clear s | ';'/pushn)

ifbody = '{' ?doc/_code '}'

swbody = *("case"! expr/_code ':' ?doc/_code)/_ARITY ?("default"! ':' ?doc/_code)/_ARITY

fnbody = '(' IDENT/name %= ','/ARITY ?"..."/ARITY ')' '{'/_mute ?doc/_code '}'/_unmute

clsname = '(' IDENT/ref ')' | IDENT/ref

newargs = ?('(' expr %= ','/ARITY ')')/ARITY

classb = "fn"! IDENT/name fnbody ?';'/mfn

atom =
	'(' expr %= ','/ARITY ?"..."/ARITY ?',' ')'/call |
	'.' (IDENT|"class"|"new"|"recover"|"main")/mref |
	'[' ?expr/ARITY ?':'/ARITY ?expr/ARITY ']'/index

factor =
	INT/pushi |
	FLOAT/pushf |
	STRING/pushs |
	CHAR/pushc |
	(IDENT/ref | '('! expr ')' | "fn"! fnbody/fn | '[' expr %= ','/ARITY ?',' ']'/slice) *atom |
	"if"/_mute! expr/_code ifbody *("elif" expr/_code ifbody)/_ARITY ?("else" ifbody)/_ARITY/_unmute/if |
	"switch"/_mute! ?(~'{' expr)/_code '{' swbody '}'/_unmute/switch |
	"for"/_mute! (~'{' s)/_code %= ';'/_ARITY '{' ?doc/_code '}'/_unmute/for |
	"new"! clsname newargs /new |
	"class"! '{' *classb/ARITY '}'/class |
	"recover" '(' ')'/recover |
	"main" '{'/_mute ?doc/_code '}'/_unmute/main |
	'{'! (expr ':' expr) %= ','/ARITY ?',' '}'/map |
	'!' factor/not |
	'-' factor/neg |
	'+' factor
`

// -----------------------------------------------------------------------------

type Compiler struct {
	Incl  func(file string) int
	code  *exec.Code
	exits []func()
	gvars map[string]interface{}
	gstk  exec.Stack
}

func includeNotimpl(file string) int {

	panic("instruction `include` not implemented")
}

func New() *Compiler {

	gvars := make(map[string]interface{})
	return &Compiler{code: exec.New(), gvars: gvars, Incl: includeNotimpl}
}

func (p *Compiler) Vars() map[string]interface{} {

	return p.gvars
}

func (p *Compiler) Code() *exec.Code {

	return p.code
}

func (p *Compiler) Grammar() string {

	return Grammar
}

func (p *Compiler) Fntable() map[string]interface{} {

	return qlang.Fntable
}

func (p *Compiler) Stack() interpreter.Stack {

	return nil
}

func (p *Compiler) VMap() {

	arity := p.popArity()
	p.code.Block(exec.Call(qlang.MapFrom, arity*2))
}

func (p *Compiler) VSlice() {

	arity := p.popArity()
	p.code.Block(exec.Call(qlang.SliceFrom, arity))
}

func (p *Compiler) VCall() {

	variadic := p.popArity()
	arity := p.popArity()
	if variadic != 0 {
		if arity == 0 {
			panic("what do you mean of `...`?")
		}
		p.code.Block(exec.CallFnv(arity))
	} else {
		p.code.Block(exec.CallFn(arity))
	}
}

func (p *Compiler) Index() {

	arity2 := p.popArity()
	arityMid := p.popArity()
	arity1 := p.popArity()

	if arityMid == 0 {
		if arity1 == 0 {
			panic("call operator[] without index")
		}
		p.code.Block(exec.Call(qlang.Get, 2))
	} else {
		p.code.Block(exec.Op3(qlang.SubSlice, arity1 != 0, arity2 != 0))
	}
}

func (p *Compiler) CodeLine(f *interpreter.FileLine) {

	p.code.CodeLine(f.File, f.Line)
}

func (p *Compiler) CallFn(fn interface{}) {

	p.code.Block(exec.Call(fn))
}

// -----------------------------------------------------------------------------

var exports = map[string]interface{}{
	"$ARITY":   (*Compiler).Arity,
	"$_ARITY":  (*Compiler).Arity,
	"$_code":   (*Compiler).PushCode,
	"$name":    (*Compiler).PushName,
	"$pushn":   (*Compiler).PushNil,
	"$pushi":   (*Compiler).PushInt,
	"$pushf":   (*Compiler).PushFloat,
	"$pushs":   (*Compiler).PushString,
	"$pushc":   (*Compiler).PushByte,
	"$index":   (*Compiler).Index,
	"$mref":    (*Compiler).MemberRef,
	"$ref":     (*Compiler).Ref,
	"$slice":   (*Compiler).VSlice,
	"$map":     (*Compiler).VMap,
	"$call":    (*Compiler).VCall,
	"$assign":  (*Compiler).Assign,
	"$massign": (*Compiler).MultiAssign,
	"$inc":     (*Compiler).Inc,
	"$dec":     (*Compiler).Dec,
	"$adda":    (*Compiler).AddAssign,
	"$suba":    (*Compiler).SubAssign,
	"$mula":    (*Compiler).MulAssign,
	"$quoa":    (*Compiler).QuoAssign,
	"$moda":    (*Compiler).ModAssign,
	"$defer":   (*Compiler).Defer,
	"$recover": (*Compiler).Recover,
	"$return":  (*Compiler).Return,
	"$fn":      (*Compiler).Function,
	"$main":    (*Compiler).Main,
	"$include": (*Compiler).Include,
	"$mfn":     (*Compiler).MemberFuncDecl,
	"$class":   (*Compiler).Class,
	"$new":     (*Compiler).New,
	"$clear":   (*Compiler).Clear,
	"$if":      (*Compiler).If,
	"$switch":  (*Compiler).Switch,
	"$for":     (*Compiler).For,
	"$and":     (*Compiler).And,
	"$or":      (*Compiler).Or,
	"$xline":   (*Compiler).CodeLine,
}

func init() {
	qlang.Import("", exports)
}

// -----------------------------------------------------------------------------