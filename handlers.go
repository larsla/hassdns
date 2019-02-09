package hassdns

import (
	"encoding/base32"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/ulule/limiter"
	"github.com/ulule/limiter/drivers/store/memory"

	"github.com/larsla/hassdns/internal/api"
	"github.com/larsla/hassdns/internal/dns"
	"github.com/larsla/hassdns/internal/keys"
	"github.com/larsla/hassdns/internal/monitoring"
	"github.com/larsla/hassdns/internal/validation"
)

var domain string
var dnsP *dns.DNSProvider
var errorReporter *monitoring.ErrorReporter
var limit *limiter.Limiter

var (
	logger    = log.New(os.Stdout, "", 0)
	errLogger = log.New(os.Stderr, "", 0)
)

func init() {

	var ok bool
	domain, ok = os.LookupEnv("DOMAIN")
	if !ok {
		errLogger.Fatal("You need to specify the DOMAIN environment variable")
	}

	var err error
	dnsP, err = dns.New(domain)
	if err != nil {
		errLogger.Fatal(err)
	}

	errorReporter, err = monitoring.NewErrorReporter("hassdns")
	if err != nil {
		errLogger.Fatal(err)
	}

	// Set request limiter to 10 requests per 5 minutes
	limit = limiter.New(memory.NewStore(), limiter.Rate{
		Period: 5 * time.Minute,
		Limit:  10,
	})

}

func sendError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)
	w.Write([]byte(err.Error()))
	errorReporter.Log(err)
}

// Update is the main endpoint
func Update(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("X-HassDNS-Version", Version)

	if r.Method != http.MethodPost {
		sendError(w, http.StatusMethodNotAllowed, errors.New("Method not allowed"))
		return
	}

	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1024*50))
	if err != nil {
		sendError(w, http.StatusInternalServerError, err)
		return
	}

	u := api.Update{}
	if err := json.Unmarshal(body, &u); err != nil {
		sendError(w, http.StatusInternalServerError, err)
		return
	}

	if len(u.Subdomain) < 4 || !validation.ValidateName(u.Subdomain) {
		sendError(w, http.StatusBadRequest, errors.New("Subdomain needs to be minimum 4 characters long and can only contain a-z and 0-9"))
		return
	}

	span := int64(300)
	now := time.Now().UTC().Unix()
	if now < u.Timestamp-span || now > u.Timestamp+span {
		sendError(w, http.StatusBadRequest, errors.New("Request timestamp invalid"))
		return
	}

	publicKey, err := keys.StringToPublicKey(u.PublicKey)
	if err != nil {
		sendError(w, http.StatusBadRequest, err)
		return
	}

	signature, err := base32.StdEncoding.DecodeString(u.Signature)
	if err != nil {
		sendError(w, http.StatusBadRequest, errors.Wrap(err, "failed to decode signature"))
		return
	}

	msg := fmt.Sprintf("%s,%d", u.Subdomain, u.Timestamp)
	if !keys.Verify(publicKey, []byte(msg), signature) {
		sendError(w, http.StatusBadRequest, errors.New("Failed to verify signature"))
		return
	}

	// Check if this publickey has reached its request limit
	l, err := limit.Get(r.Context(), u.PublicKey)
	if err != nil {
		sendError(w, http.StatusInternalServerError, errors.Wrap(err, "failed to get limit"))
		return
	}
	if l.Reached {
		sendError(w, http.StatusTooManyRequests, errors.New("Too many requests"))
		return
	}

	fqdn := fmt.Sprintf("%s.%s.", string(u.Subdomain), domain)

	rrs, err := dnsP.GetRRs(fqdn)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err)
		return
	}

	oldValue := ""
	if len(rrs) > 0 {
		foundTxt := false
		for _, rr := range rrs {
			if rr.Type == "TXT" && len(rr.Rrdatas) > 0 {
				foundTxt = true
				logger.Printf("Name %s owned by key '%s'", fqdn, rr.Rrdatas[0])
				if strings.Trim(rr.Rrdatas[0], "\"") != u.PublicKey {
					sendError(w, http.StatusForbidden, errors.New("Name owned by another key"))
					return
				}
			}
			if rr.Type == "A" && len(rr.Rrdatas) > 0 {
				oldValue = strings.Trim(rr.Rrdatas[0], "\"")
			}
		}
		if !foundTxt {
			sendError(w, http.StatusUnauthorized, errors.New("Looks like a reserved entry"))
			return
		}
	} else {
		logger.Printf("Adding new owner of %s: '%s", fqdn, u.PublicKey)
		if err := dnsP.Update(fqdn, "TXT", u.PublicKey, 300); err != nil {
			sendError(w, http.StatusInternalServerError, err)
			return
		}
	}

	newValue := r.Header.Get("X-Forwarded-For")
	if newValue != oldValue {
		if err := dnsP.Update(fqdn, "A", r.Header.Get("X-Forwarded-For"), 60); err != nil {
			sendError(w, http.StatusInternalServerError, err)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Successfully updated %s", fqdn)))
}
