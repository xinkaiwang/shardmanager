package kerror

type ErrorCode string

var (
	httpErrorCodeMap = createMapHttpErrorCode()
)

const (
	EC_OK                ErrorCode = "OK"
	EC_UNKNOWN           ErrorCode = "UNKNOWN"
	EC_NOT_FOUND         ErrorCode = "NOT_FOUND"
	EC_INVALID_PARAMETER ErrorCode = "INVALID_PARAMETER"
	EC_CONFLICT          ErrorCode = "CONFLICT"
	EC_INTERNAL_ERROR    ErrorCode = "INTERNAL_ERROR"
	EC_UNIMPLEMENTED     ErrorCode = "UNIMPLEMENTED"
	EC_TIMEOUT           ErrorCode = "TIMEOUT"
	EC_NETWORK_ERR       ErrorCode = "NETWORK_ERR"
	EC_RETRYABLE         ErrorCode = "RETRYABLE"
	EC_UNAUTHENTICATED   ErrorCode = "UNAUTHENTICATED"
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
	return dict
}
