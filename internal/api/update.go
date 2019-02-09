package api

type Update struct {
	Timestamp int64  `json:"timestamp"`
	PublicKey string `json:"public_key"`
	Subdomain string `json:"subdomain"`
	Signature string `json:"signature"`
}
