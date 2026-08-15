package httphandlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	serviceauth "github.com/futrx-com/remote.futrx.com/internal/service/auth"
	servicepush "github.com/futrx-com/remote.futrx.com/internal/service/push"
	httptransport "github.com/futrx-com/remote.futrx.com/internal/transport/http"
)

// PushHandler exposes the browser side of Web Push: the application server key
// to subscribe with, and this user's device registrations.
type PushHandler struct {
	push *servicepush.Service
	auth *serviceauth.Service
}

func NewPushHandler(push *servicepush.Service, auth *serviceauth.Service) *PushHandler {
	return &PushHandler{push: push, auth: auth}
}

func (h *PushHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/push/config", h.HandleConfig)
	mux.HandleFunc("/api/push/subscriptions", h.HandleSubscriptions)
	mux.HandleFunc("/api/push/test", h.HandleTest)
}

type pushConfigResponse struct {
	Enabled    bool   `json:"enabled"`
	PublicKey  string `json:"publicKey,omitempty"`
	Subscribed bool   `json:"subscribed"`
}

// subscriptionRequest mirrors the shape of PushSubscription.toJSON() so the
// browser can post its subscription unmodified.
type subscriptionRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

func (h *PushHandler) HandleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	email, err := h.caller(r)
	if err != nil {
		httptransport.SendErr(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if !h.push.Enabled() {
		httptransport.SendJSON(w, http.StatusOK, pushConfigResponse{})
		return
	}

	subscribed, err := h.push.HasSubscriptions(r.Context(), email)
	if err != nil {
		httptransport.SendErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	httptransport.SendJSON(w, http.StatusOK, pushConfigResponse{
		Enabled:    true,
		PublicKey:  h.push.PublicKey(),
		Subscribed: subscribed,
	})
}

func (h *PushHandler) HandleSubscriptions(w http.ResponseWriter, r *http.Request) {
	email, err := h.caller(r)
	if err != nil {
		httptransport.SendErr(w, http.StatusUnauthorized, "authentication required")
		return
	}

	switch r.Method {
	case http.MethodPost:
		var body subscriptionRequest
		if err := decodePushBody(r, &body); err != nil {
			httptransport.SendErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		err := h.push.Subscribe(r.Context(), email, servicepush.Subscription{
			Endpoint:  body.Endpoint,
			P256dh:    body.Keys.P256dh,
			Auth:      body.Keys.Auth,
			UserAgent: r.UserAgent(),
		})
		if err != nil {
			sendPushError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	case http.MethodDelete:
		var body subscriptionRequest
		if err := decodePushBody(r, &body); err != nil {
			httptransport.SendErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		if err := h.push.Unsubscribe(r.Context(), email, body.Endpoint); err != nil {
			sendPushError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// HandleTest delivers one notification to the caller's own devices, which is
// the only way to tell a broken subscription from a quiet agent.
func (h *PushHandler) HandleTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	email, err := h.caller(r)
	if err != nil {
		httptransport.SendErr(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if !h.push.Enabled() {
		sendPushError(w, servicepush.ErrDisabled)
		return
	}

	h.push.Notify(r.Context(), []string{email}, servicepush.Notification{
		Kind:  servicepush.KindTest,
		Title: "Notifications are working",
		Body:  "You will be notified when an agent asks a question or finishes a turn.",
		Tag:   "push-test",
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *PushHandler) caller(r *http.Request) (string, error) {
	if h.auth == nil {
		return "local-admin", nil
	}
	session, err := httptransport.NewPrincipalResolver(h.auth).Session(r)
	if err != nil {
		return "", err
	}
	if session == nil {
		return "", errors.New("no session")
	}
	return session.Email, nil
}

func decodePushBody(r *http.Request, target any) error {
	err := json.NewDecoder(io.LimitReader(r.Body, 1<<15)).Decode(target)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func sendPushError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, servicepush.ErrDisabled):
		httptransport.SendErr(w, http.StatusServiceUnavailable, "push notifications are not configured")
	case errors.Is(err, servicepush.ErrInvalidIdentity):
		httptransport.SendErr(w, http.StatusUnauthorized, "authentication required")
	case errors.Is(err, servicepush.ErrInvalidEndpoint),
		errors.Is(err, servicepush.ErrInvalidKeys):
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, servicepush.ErrTooManySubscription):
		httptransport.SendErr(w, http.StatusConflict, err.Error())
	default:
		httptransport.SendErr(w, http.StatusInternalServerError, err.Error())
	}
}
