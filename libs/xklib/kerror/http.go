package kerror

// FromHTTPStatus converts HTTP status code to ErrorCode (best effort).
func FromHTTPStatus(status int) ErrorCode {
	if code, ok := httpToErrorCodeMap[status]; ok {
		return code
	}

	// Fallback based on status code ranges
	switch {
	case status >= 500:
		return EC_INTERNAL
	case status >= 400:
		return EC_INVALID_ARGUMENT
	default:
		return EC_UNKNOWN
	}
}

// createMapHttpToErrorCode creates the reverse mapping: HTTP status → ErrorCode.
// This is a best-effort mapping since HTTP has fewer status codes than gRPC.
func createMapHttpToErrorCode() map[int]ErrorCode {
	dict := map[int]ErrorCode{}

	dict[200] = EC_OK
	dict[400] = EC_INVALID_ARGUMENT
	dict[401] = EC_UNAUTHENTICATED
	dict[403] = EC_PERMISSION_DENIED
	dict[404] = EC_NOT_FOUND
	dict[409] = EC_ALREADY_EXISTS // Could also be ABORTED, but ALREADY_EXISTS is more common
	dict[412] = EC_FAILED_PRECONDITION
	dict[429] = EC_RESOURCE_EXHAUSTED
	dict[499] = EC_CANCELLED // nginx convention
	dict[500] = EC_INTERNAL
	dict[501] = EC_UNIMPLEMENTED
	dict[503] = EC_UNAVAILABLE
	dict[504] = EC_DEADLINE_EXCEEDED

	return dict
}
