package azure

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

// Forbidden reports whether err is an Azure answer with status 403.
func Forbidden(err error) bool {
	var re *azcore.ResponseError
	return errors.As(err, &re) && re.StatusCode == http.StatusForbidden
}

// ErrorText is the status and error code of an Azure answer, such as
// "400 BadRequest", else the text of err. The text of an answer or of a
// transport error holds the request URL.
func ErrorText(err error) string {
	var re *azcore.ResponseError
	if !errors.As(err, &re) {
		var ue *url.Error
		if errors.As(err, &ue) {
			return ue.Err.Error()
		}
		return err.Error()
	}
	return strings.TrimSpace(strconv.Itoa(re.StatusCode) + " " + re.ErrorCode)
}
