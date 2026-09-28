package service

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
	"time"
)

type smtpMessage struct {
	envelopeFrom string
	envelopeTo   string
	data         []byte
}

func buildSMTPMessage(config *SMTPConfig, to, subject, body string) (smtpMessage, error) {
	return buildSMTPMessageWithAttachment(config, to, subject, body, "", nil)
}

func buildSMTPMessageWithAttachment(config *SMTPConfig, to, subject, body, filename string, attachment []byte) (smtpMessage, error) {
	if config == nil {
		return smtpMessage{}, errors.New("missing SMTP configuration")
	}

	fromAddress, err := parseSMTPAddress(config.From, "from")
	if err != nil {
		return smtpMessage{}, err
	}
	recipientAddress, err := parseSMTPAddress(to, "recipient")
	if err != nil {
		return smtpMessage{}, err
	}
	messageID, err := generateEmailMessageID(fromAddress.Address, config.Host)
	if err != nil {
		return smtpMessage{}, fmt.Errorf("generate message ID: %w", err)
	}

	fromName := sanitizeEmailHeader(config.FromName)
	if strings.TrimSpace(fromName) == "" {
		fromName = fromAddress.Name
	}
	fromHeader := (&mail.Address{
		Name:    fromName,
		Address: fromAddress.Address,
	}).String()
	toHeader := (&mail.Address{
		Name:    recipientAddress.Name,
		Address: recipientAddress.Address,
	}).String()
	subjectHeader := mime.QEncoding.Encode("UTF-8", sanitizeEmailHeader(subject))

	var message bytes.Buffer
	fmt.Fprintf(&message, "From: %s\r\n", fromHeader)
	fmt.Fprintf(&message, "To: %s\r\n", toHeader)
	fmt.Fprintf(&message, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&message, "Message-ID: %s\r\n", messageID)
	fmt.Fprintf(&message, "Subject: %s\r\n", subjectHeader)
	fmt.Fprint(&message, "MIME-Version: 1.0\r\n")
	if attachment == nil {
		fmt.Fprint(&message, "Content-Type: text/html; charset=UTF-8\r\n"+
			"Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		bodyWriter := quotedprintable.NewWriter(&message)
		if _, err := bodyWriter.Write([]byte(body)); err != nil {
			return smtpMessage{}, fmt.Errorf("encode email body: %w", err)
		}
		if err := bodyWriter.Close(); err != nil {
			return smtpMessage{}, fmt.Errorf("close email body encoder: %w", err)
		}
	} else {
		if strings.ContainsAny(filename, "\r\n") || filename == "" {
			return smtpMessage{}, errors.New("invalid attachment filename")
		}
		var parts bytes.Buffer
		writer := multipart.NewWriter(&parts)
		bodyHeader := textproto.MIMEHeader{}
		bodyHeader.Set("Content-Type", "text/html; charset=UTF-8")
		bodyHeader.Set("Content-Transfer-Encoding", "quoted-printable")
		bodyPart, err := writer.CreatePart(bodyHeader)
		if err != nil {
			return smtpMessage{}, err
		}
		bodyWriter := quotedprintable.NewWriter(bodyPart)
		if _, err := bodyWriter.Write([]byte(body)); err != nil {
			return smtpMessage{}, err
		}
		if err := bodyWriter.Close(); err != nil {
			return smtpMessage{}, err
		}
		fileHeader := textproto.MIMEHeader{}
		fileHeader.Set("Content-Type", "application/pdf")
		fileHeader.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
		fileHeader.Set("Content-Transfer-Encoding", "base64")
		filePart, err := writer.CreatePart(fileHeader)
		if err != nil {
			return smtpMessage{}, err
		}
		for offset := 0; offset < len(attachment); offset += 57 {
			end := offset + 57
			if end > len(attachment) {
				end = len(attachment)
			}
			line := base64.StdEncoding.EncodeToString(attachment[offset:end])
			if _, err := fmt.Fprintf(filePart, "%s\r\n", line); err != nil {
				return smtpMessage{}, err
			}
		}
		if err := writer.Close(); err != nil {
			return smtpMessage{}, err
		}
		fmt.Fprintf(&message, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n", writer.Boundary())
		message.Write(parts.Bytes())
	}

	return smtpMessage{
		envelopeFrom: fromAddress.Address,
		envelopeTo:   recipientAddress.Address,
		data:         message.Bytes(),
	}, nil
}

func parseSMTPAddress(value, field string) (*mail.Address, error) {
	if strings.ContainsAny(value, "\r\n") {
		return nil, fmt.Errorf("invalid SMTP %s address: contains a line break", field)
	}

	cleaned := strings.TrimSpace(value)
	address, err := mail.ParseAddress(cleaned)
	if err != nil || strings.TrimSpace(address.Address) == "" {
		if err == nil {
			err = fmt.Errorf("address is empty")
		}
		return nil, fmt.Errorf("invalid SMTP %s address: %w", field, err)
	}
	return address, nil
}

func generateEmailMessageID(fromAddress, smtpHost string) (string, error) {
	randomID := make([]byte, 16)
	if _, err := rand.Read(randomID); err != nil {
		return "", err
	}

	domain := strings.TrimSpace(sanitizeEmailHeader(smtpHost))
	if at := strings.LastIndexByte(fromAddress, '@'); at >= 0 && at < len(fromAddress)-1 {
		domain = fromAddress[at+1:]
	}
	domain = strings.Trim(domain, "[]<>")
	if domain == "" {
		domain = "localhost"
	}

	return fmt.Sprintf("<%s@%s>", hex.EncodeToString(randomID), domain), nil
}
