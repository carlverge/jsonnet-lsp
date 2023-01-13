package linter

type DiagCode string

const (
	ImportNotFound      DiagCode = "ImportNotFound"
	UnusedVar           DiagCode = "UnusedVar"
	TypeMismatch        DiagCode = "TypeMismatch"
	RedundantCondition  DiagCode = "RedundantCondition"
	UnknownField        DiagCode = "UnknownField"
	UnknownArgument     DiagCode = "UnknownArgument"
	ArgumentCardinality DiagCode = "ArgumentCardinality"
)
