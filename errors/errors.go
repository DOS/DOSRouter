// Package errors provides typed error types for blockchain wallet and RPC
// failures in the DOSRouter system.
package errors

import (
	stderrors "errors"
	"fmt"
)

// InsufficientFundsError indicates the wallet balance is too low for the
// requested operation.
type InsufficientFundsError struct {
	Code              string  `json:"code"`
	CurrentBalanceUSD float64 `json:"currentBalanceUSD"`
	RequiredUSD       float64 `json:"requiredUSD"`
	WalletAddress     string  `json:"walletAddress"`
}

func (e *InsufficientFundsError) Error() string {
	return fmt.Sprintf(
		"insufficient funds: wallet %s has $%.4f but $%.4f required",
		e.WalletAddress, e.CurrentBalanceUSD, e.RequiredUSD,
	)
}

// NewInsufficientFundsError creates an InsufficientFundsError.
func NewInsufficientFundsError(wallet string, current, required float64) *InsufficientFundsError {
	return &InsufficientFundsError{
		Code:              "INSUFFICIENT_FUNDS",
		CurrentBalanceUSD: current,
		RequiredUSD:       required,
		WalletAddress:     wallet,
	}
}

// EmptyWalletError indicates the wallet has a zero balance.
type EmptyWalletError struct {
	Code          string `json:"code"`
	WalletAddress string `json:"walletAddress"`
}

func (e *EmptyWalletError) Error() string {
	return fmt.Sprintf("empty wallet: %s has zero balance", e.WalletAddress)
}

// NewEmptyWalletError creates an EmptyWalletError.
func NewEmptyWalletError(wallet string) *EmptyWalletError {
	return &EmptyWalletError{
		Code:          "EMPTY_WALLET",
		WalletAddress: wallet,
	}
}

// RpcError wraps a low-level RPC failure.
type RpcError struct {
	Code          string `json:"code"`
	OriginalError error  `json:"-"`
}

func (e *RpcError) Error() string {
	if e.OriginalError != nil {
		return fmt.Sprintf("RPC error: %v", e.OriginalError)
	}
	return "RPC error"
}

func (e *RpcError) Unwrap() error {
	return e.OriginalError
}

// NewRpcError creates an RpcError wrapping the original error.
func NewRpcError(original error) *RpcError {
	return &RpcError{
		Code:          "RPC_ERROR",
		OriginalError: original,
	}
}

// ---------- Type guard functions ----------

// IsInsufficientFundsError reports whether err is an InsufficientFundsError.
func IsInsufficientFundsError(err error) (*InsufficientFundsError, bool) {
	var target *InsufficientFundsError
	if stderrors.As(err, &target) {
		return target, true
	}
	return nil, false
}

// IsEmptyWalletError reports whether err is an EmptyWalletError.
func IsEmptyWalletError(err error) (*EmptyWalletError, bool) {
	var target *EmptyWalletError
	if stderrors.As(err, &target) {
		return target, true
	}
	return nil, false
}

// IsBalanceError returns true if err is either InsufficientFundsError or EmptyWalletError.
func IsBalanceError(err error) bool {
	if _, ok := IsInsufficientFundsError(err); ok {
		return true
	}
	if _, ok := IsEmptyWalletError(err); ok {
		return true
	}
	return false
}

// IsRpcError reports whether err is an RpcError.
func IsRpcError(err error) (*RpcError, bool) {
	var target *RpcError
	if stderrors.As(err, &target) {
		return target, true
	}
	return nil, false
}
