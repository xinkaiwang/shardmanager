package kerror

type ErrorCode string

const (
	EC_OK                ErrorCode = "OK"
	EC_UNKNOWN           ErrorCode = "UNKNOWN"
	EC_NOT_FOUND         ErrorCode = "NOT_FOUND"
	EC_INVALID_PARAMETER ErrorCode = "INVALID_PARAMETER"
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
