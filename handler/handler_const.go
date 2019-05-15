package handler

type contextKey int

const (
	contextUserKey contextKey = iota
	contextSessionKey
)
