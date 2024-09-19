package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"log"
	"strings"
	"sync/atomic"
)

var (
	ErrUserExceedingQuota = errors.New("user is exceeding quota")
	ErrUnauthorized       = errors.New("unauthorized")
)

func NewAccountingHandler(users ...UserConfig) *AccountingHandler {
	return &AccountingHandler{
		users:           users,
		downstreamBytes: make([]atomic.Int64, len(users)),
	}
}

type AccountingHandler struct {
	SimpleHandler

	users           []UserConfig
	downstreamBytes []atomic.Int64
}

func (ah *AccountingHandler) SendUpstream(ctx ConnContext, w io.Writer, r io.Reader) error {
	username, password, err := userPassFromBearer(ctx.ProxyAuthorization)
	if err != nil {
		return err
	}

	if userIdx, ok := ah.authenticate(username, password); ok {
		if ah.downstreamBytes[userIdx].Load() >= int64(ah.users[userIdx].QuotaBytes) {
			return ErrUserExceedingQuota
		}
		return ah.SimpleHandler.SendUpstream(ctx, w, r)
	}

	return ErrUnauthorized
}

func (ah *AccountingHandler) SendDownstream(ctx ConnContext, w io.Writer, r io.Reader) error {
	username, password, err := userPassFromBearer(ctx.ProxyAuthorization)
	if err != nil {
		return err
	}

	if userIdx, ok := ah.authenticate(username, password); ok {
		return copyAndAccount(w, r, &ah.downstreamBytes[userIdx])
	}

	return ErrUnauthorized
}

func (ah *AccountingHandler) authenticate(username, password string) (int, bool) {
	for idx, user := range ah.users {
		if user.Username == username && user.Password == password {
			log.Printf("Authenticated user: %s", username)
			return idx, true
		}
	}
	return 0, false
}

func userPassFromBearer(bearer string) (string, string, error) {
	_, encoded, found := strings.Cut(bearer, "Basic ")
	if !found {
		return "", "", ErrUnauthorized
	}

	decoder := base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(encoded))
	decoded, err := io.ReadAll(decoder)
	if err != nil {
		return "", "", err
	}

	username, password, found := strings.Cut(string(decoded), ":")
	if !found {
		return "", "", ErrUnauthorized
	}

	return username, password, nil
}

func copyAndAccount(w io.Writer, r io.Reader, counter *atomic.Int64) error {
	const bufferSize = 5 * 1024
	buffer := make([]byte, bufferSize)

	for {

		var (
			bytesWritten, bytesRead, bytesAdded int
			readErr, writeErr                   error
		)
		bytesRead, readErr = r.Read(buffer)
		if bytesRead > 0 {
			for bytesWritten < bytesRead {
				bytesAdded, writeErr = w.Write(buffer[0:bytesRead])
				if bytesAdded > 0 {
					bytesWritten += bytesAdded
					counter.Add(int64(bytesAdded))
				}
				if writeErr != nil {
					return writeErr
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		} else if readErr != nil {
			return readErr
		}
	}
}
