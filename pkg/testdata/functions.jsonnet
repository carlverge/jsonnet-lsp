local tooFewArgs = std.endsWith('');
local tooManyArgs = std.endsWith('', '', '');
local argWrongType = std.all(5);
local notAFunc = "asdf";
local callingNonFunc = notAFunc(2);
local fnWithNamed(a=null, b=null) = null;
local callingDupNamedArgs = fnWithNamed(a=2, a=3);
local typedFn(a/*:string*/, b/*:number*/, c=null) = null;
local wrongTypeHintArg = typedFn(2, false);

{used: [tooFewArgs, tooManyArgs, argWrongType, callingNonFunc, callingDupNamedArgs, wrongTypeHintArg]}