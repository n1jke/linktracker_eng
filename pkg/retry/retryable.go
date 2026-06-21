package retry

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func NewGRPCRetryableFunc(retrCodes []uint32) func(error) bool {
	retryable := make(map[codes.Code]struct{}, len(retrCodes))
	for i := range retrCodes {
		retryable[codes.Code(retrCodes[i])] = struct{}{}
	}

	return func(err error) bool {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}

		st, ok := status.FromError(err)
		if !ok {
			return true
		}

		_, ex := retryable[st.Code()]

		return ex
	}
}

func NewHTTPRetryableFunc(retrCodes []int) func(int, error) bool {
	retryable := make(map[int]struct{}, len(retrCodes))
	for i := range retrCodes {
		retryable[retrCodes[i]] = struct{}{}
	}

	return func(code int, err error) bool {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}

		if err != nil {
			return true
		}

		_, ex := retryable[code]

		return ex
	}
}
