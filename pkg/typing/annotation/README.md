
## Type Comments

In a function:
```
function(a/*:type*/, b/*:type*/, c/*:type*/=null) /*:return*/
```

In an object definition:
```
{
    a: /*:type*/ null,
    b: /*:type*/ null,
    fn(a/*:type*/, b/*:type*/, c/*:type*/=null): /*:return*/
        null,
}
```

## Examples

```jsonnet
local joinStr(arr/*:array[string]*/, sep/*:string*/) /*:string*/ =
    std.join(arr, sep);

// If widget is used as a template, there will be a warning if a is not overridden
local Widget = {
    a: /*:number*/ null,
    a: /*:number | null*/ null,
};

local addWidget(w/*:Widget*/, x/*:number*/) /*:number*/ = 
    x + w.a + (if w.b == null then 0 else w.b);
```

Local variables are inferred from the result of their assignment.
* Caveat: extvars, maybe imports?

## Type Format

```
ident
dotted-var := ident ( "." ident )+
type-atom := 
    bool-lit | 
    number-lit | 
    string-lit |
    any |
    null |
    boolean | 
    string | 
    number | 
    function | 
    array ( "[" type-decl "]") | 
    object ( "[" type-decl "]") |
    dotted-var
type-decl := type-atom ( "|" type-atom )+
```

* For array and object, they optionally can have an element type specified like `array[string]` or `object[null | string]`.
* If a variable is referenced, it must:
    * Be a function, whose signature will be used as a function template.
    * Be an object, whose shape will be used as a struct signature.
* If a literal is specified, the inputs must match that value.


## Type Parameters

```
function(a: A, b: array[B]) -> B

// Not allowed for now, no type params in unions
function(a: A | B) -> A

function(fn: function(agg: T, elem: E) -> T, arr: array[E], init: T) -> T

function(arr: array[E], keyFn: function(E) -> string) -> object[array[E]]
```

#### Rules
* No type parameters in unions (or types inside unions)
* Non-function parameters can only include one type parameter (same with return types)
* A type parameter in the return must also exist in the parameters
* 


## Evaluation

* Treat everything (including conditionals, binops, unops) as functions
* evaluate the typehints and inferred types in parallel
* lint warning if the typehints across values don't match
* lint warning if the inferred type and typehint on a single value don't match
* for refinement, add overlay on varmap that changes variable types