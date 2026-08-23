package model

import "errors"

// 领域级错误：HTTP 层据此映射状态码与用户可读信息。
var (
	// ErrNotFound 资源不存在。
	ErrNotFound = errors.New("not found")
	// ErrConflict 唯一约束 / 幂等冲突（如指纹重复且语义不一致）。
	ErrConflict = errors.New("conflict")
	// ErrInvalidState 状态机非法流转（如冻结结果直接编辑）。
	ErrInvalidState = errors.New("invalid state transition")
	// ErrInsufficientData 数据不足（如空视野、校准样本不足）。
	ErrInsufficientData = errors.New("insufficient data")
	// ErrInvalidArgument 参数非法（单位混用、角度越界等）。
	ErrInvalidArgument = errors.New("invalid argument")
	// ErrBadInput 输入格式错误。
	ErrBadInput = errors.New("bad input")
)
