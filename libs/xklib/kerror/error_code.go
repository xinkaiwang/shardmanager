package kerror

type ErrorCode string

var (
	httpErrorCodeMap   = createMapHttpErrorCode()
	httpToErrorCodeMap = createMapHttpToErrorCode()
)

// ErrorCode constants based on gRPC status codes.
// See: https://github.com/googleapis/googleapis/blob/master/google/rpc/code.proto
const (
	// Original codes (kept for backward compatibility)
	EC_OK                ErrorCode = "OK"
	EC_UNKNOWN           ErrorCode = "UNKNOWN"
	EC_NOT_FOUND         ErrorCode = "NOT_FOUND"
	EC_INVALID_PARAMETER ErrorCode = "INVALID_PARAMETER" // alias for INVALID_ARGUMENT
	EC_CONFLICT          ErrorCode = "CONFLICT"          // alias for ALREADY_EXISTS
	EC_INTERNAL_ERROR    ErrorCode = "INTERNAL_ERROR"    // alias for INTERNAL
	EC_UNIMPLEMENTED     ErrorCode = "UNIMPLEMENTED"
	EC_TIMEOUT           ErrorCode = "TIMEOUT"     // alias for DEADLINE_EXCEEDED
	EC_NETWORK_ERR       ErrorCode = "NETWORK_ERR" // alias for UNAVAILABLE
	EC_RETRYABLE         ErrorCode = "RETRYABLE"   // custom code
	EC_UNAUTHENTICATED   ErrorCode = "UNAUTHENTICATED"

	// Complete gRPC code set (new additions)
	EC_CANCELLED           ErrorCode = "CANCELLED"
	EC_INVALID_ARGUMENT    ErrorCode = "INVALID_ARGUMENT"
	EC_DEADLINE_EXCEEDED   ErrorCode = "DEADLINE_EXCEEDED"
	EC_ALREADY_EXISTS      ErrorCode = "ALREADY_EXISTS"
	EC_PERMISSION_DENIED   ErrorCode = "PERMISSION_DENIED"
	EC_RESOURCE_EXHAUSTED  ErrorCode = "RESOURCE_EXHAUSTED"
	EC_FAILED_PRECONDITION ErrorCode = "FAILED_PRECONDITION"
	EC_ABORTED             ErrorCode = "ABORTED"
	EC_OUT_OF_RANGE        ErrorCode = "OUT_OF_RANGE"
	EC_INTERNAL            ErrorCode = "INTERNAL"
	EC_UNAVAILABLE         ErrorCode = "UNAVAILABLE"
	EC_DATA_LOSS           ErrorCode = "DATA_LOSS"
)

func (code ErrorCode) String() string {
	return string(code)
}

func (ec ErrorCode) ToHttpErrorCode() int {
	code, ok := httpErrorCodeMap[ec]
	if ok {
		return code
	}
	return 503
}

func createMapHttpErrorCode() map[ErrorCode]int {
	dict := map[ErrorCode]int{}

	// Original mappings (backward compatibility)
	dict[EC_OK] = 200
	dict[EC_UNKNOWN] = 500
	dict[EC_NOT_FOUND] = 404
	dict[EC_INVALID_PARAMETER] = 400
	dict[EC_CONFLICT] = 409
	dict[EC_INTERNAL_ERROR] = 503
	dict[EC_UNIMPLEMENTED] = 501
	dict[EC_TIMEOUT] = 408
	dict[EC_NETWORK_ERR] = 504
	dict[EC_RETRYABLE] = 429
	dict[EC_UNAUTHENTICATED] = 401

	// Complete gRPC → HTTP mappings
	dict[EC_CANCELLED] = 499           // Client Closed Request (nginx convention)
	dict[EC_INVALID_ARGUMENT] = 400    // Bad Request
	dict[EC_DEADLINE_EXCEEDED] = 504   // Gateway Timeout
	dict[EC_ALREADY_EXISTS] = 409      // Conflict
	dict[EC_PERMISSION_DENIED] = 403   // Forbidden
	dict[EC_RESOURCE_EXHAUSTED] = 429  // Too Many Requests
	dict[EC_FAILED_PRECONDITION] = 412 // Precondition Failed
	dict[EC_ABORTED] = 409             // Conflict
	dict[EC_OUT_OF_RANGE] = 400        // Bad Request
	dict[EC_INTERNAL] = 500            // Internal Server Error
	dict[EC_UNAVAILABLE] = 503         // Service Unavailable
	dict[EC_DATA_LOSS] = 500           // Internal Server Error

	return dict
}
