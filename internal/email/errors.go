package email

import (
	"context"
	"crypto/x509"
	"errors"
	"net"
	"net/textproto"
)

type FailureClass string

const (
	FailureTransient FailureClass = "transient"
	FailurePermanent FailureClass = "permanent"
)

type DeliveryError struct {
	Class  FailureClass
	Reason string
	Err    error
}

func (e *DeliveryError) Error() string {
	if e == nil || e.Err == nil {
		return "email delivery failed"
	}
	return e.Err.Error()
}

func (e *DeliveryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func TransientError(reason string, err error) error {
	return &DeliveryError{Class: FailureTransient, Reason: reason, Err: err}
}

func PermanentError(reason string, err error) error {
	return &DeliveryError{Class: FailurePermanent, Reason: reason, Err: err}
}

func ClassifyFailure(err error) (FailureClass, string) {
	if err == nil {
		return "", ""
	}

	var deliveryErr *DeliveryError
	if errors.As(err, &deliveryErr) {
		return deliveryErr.Class, deliveryErr.Reason
	}

	var smtpErr *textproto.Error
	if errors.As(err, &smtpErr) {
		if smtpErr.Code >= 500 && smtpErr.Code <= 599 {
			return FailurePermanent, "smtp_permanent_failure"
		}
		if smtpErr.Code >= 400 && smtpErr.Code <= 499 {
			return FailureTransient, "smtp_transient_failure"
		}
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return FailureTransient, "delivery_cancelled"
	}

	var networkErr net.Error
	if errors.As(err, &networkErr) {
		return FailureTransient, "smtp_unavailable"
	}

	// Unknown failures are retried within the configured attempt and delivery
	// bounds so a new provider error cannot silently discard email.
	return FailureTransient, "delivery_error"
}

func classifySMTPError(reason string, err error) error {
	var smtpErr *textproto.Error
	if errors.As(err, &smtpErr) && smtpErr.Code >= 500 && smtpErr.Code <= 599 {
		return PermanentError(reason, err)
	}
	return TransientError(reason, err)
}

func classifySMTPAuthError(err error) error {
	var smtpErr *textproto.Error
	if errors.As(err, &smtpErr) {
		return classifySMTPError("smtp_authentication_failed", err)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return TransientError("smtp_authentication_failed", err)
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) {
		return TransientError("smtp_authentication_failed", err)
	}
	return PermanentError("smtp_authentication_failed", err)
}

func classifySMTPTLSError(err error) error {
	var unknownAuthority x509.UnknownAuthorityError
	var hostnameError x509.HostnameError
	var certificateInvalid x509.CertificateInvalidError
	if errors.As(err, &unknownAuthority) || errors.As(err, &hostnameError) || errors.As(err, &certificateInvalid) {
		return PermanentError("smtp_tls_failed", err)
	}
	return classifySMTPError("smtp_tls_failed", err)
}
