package upnp

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const soapTimeout = 10 * time.Second

// UPnPError is the fault a gateway returns when it refuses an action.
type UPnPError struct {
	Action      string
	Code        int
	Description string
}

func (e *UPnPError) Error() string {
	return fmt.Sprintf("upnp: %s rejected with error %d (%s)", e.Action, e.Code, e.Description)
}

const (
	errorCodeConflictInMappingEntry   = 718
	errorCodeOnlyPermanentLeases      = 725
	errorCodeNoSuchEntryInArray       = 714
	errorCodeSpecifiedArrayIndexValid = 713
)

type soapArgument struct {
	Name  string
	Value string
}

func (g *Gateway) call(ctx context.Context, action string, arguments []soapArgument) (map[string]string, error) {
	body := buildEnvelope(g.ServiceType, action, arguments)

	requestCtx, cancel := context.WithTimeout(ctx, soapTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, g.ControlURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("upnp: cannot build %s request: %w", action, err)
	}
	request.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	request.Header.Set("SOAPAction", fmt.Sprintf(`"%s#%s"`, g.ServiceType, action))
	request.Header.Set("Connection", "close")

	response, err := g.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("upnp: %s call failed: %w", action, err)
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("upnp: cannot read %s response: %w", action, err)
	}

	values, fault := parseResponse(payload)
	if fault != nil {
		fault.Action = action
		return nil, fault
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upnp: %s call returned %s", action, response.Status)
	}
	return values, nil
}

func buildEnvelope(serviceType, action string, arguments []soapArgument) []byte {
	var body strings.Builder
	body.WriteString(`<?xml version="1.0"?>`)
	body.WriteString(`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body>`)
	fmt.Fprintf(&body, `<u:%s xmlns:u="%s">`, action, serviceType)
	for _, argument := range arguments {
		fmt.Fprintf(&body, "<%s>%s</%s>", argument.Name, escapeXML(argument.Value), argument.Name)
	}
	fmt.Fprintf(&body, `</u:%s>`, action)
	body.WriteString(`</s:Body></s:Envelope>`)
	return []byte(body.String())
}

func escapeXML(value string) string {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(value))
	return escaped.String()
}

// parseResponse flattens every leaf element of the SOAP body into a map, which
// keeps a single decoder valid for every action this package issues.
func parseResponse(payload []byte) (map[string]string, *UPnPError) {
	decoder := xml.NewDecoder(bytes.NewReader(payload))
	values := map[string]string{}
	isFault := false
	currentName := ""

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			break
		}
		switch element := token.(type) {
		case xml.StartElement:
			currentName = element.Name.Local
			if currentName == "Fault" || currentName == "UPnPError" {
				isFault = true
			}
		case xml.CharData:
			text := strings.TrimSpace(string(element))
			if text != "" && currentName != "" {
				values[currentName] = text
			}
		case xml.EndElement:
			currentName = ""
		}
	}

	if !isFault {
		return values, nil
	}
	code, _ := strconv.Atoi(values["errorCode"])
	description := values["errorDescription"]
	if description == "" {
		description = values["faultstring"]
	}
	return values, &UPnPError{Code: code, Description: description}
}

func errorCode(err error) int {
	var upnpErr *UPnPError
	if errors.As(err, &upnpErr) {
		return upnpErr.Code
	}
	return 0
}
