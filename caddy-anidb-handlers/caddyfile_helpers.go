package errorbodystatus

import (
	"strconv"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
)

func parseStringArg(d *caddyfile.Dispenser, name string) (string, error) {
	if !d.NextArg() {
		return "", d.ArgErr()
	}
	return d.Val(), nil
}

func parseKeywordArg(d *caddyfile.Dispenser, name string, allowed ...string) (string, error) {
	val, err := parseStringArg(d, name)
	if err != nil {
		return "", err
	}
	for _, candidate := range allowed {
		if val == candidate {
			return val, nil
		}
	}
	return "", d.Errf("%s must be one of %s", name, strings.Join(allowed, ", "))
}

func parseIntArg(d *caddyfile.Dispenser, name string) (int, error) {
	val, err := parseStringArg(d, name)
	if err != nil {
		return 0, err
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return 0, d.Errf("%s must be an integer: %v", name, err)
	}
	return parsed, nil
}

func parseFloatArg(d *caddyfile.Dispenser, name string) (float64, error) {
	val, err := parseStringArg(d, name)
	if err != nil {
		return 0, err
	}
	parsed, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, d.Errf("%s must be a valid number: %v", name, err)
	}
	return parsed, nil
}

func parseDurationArg(d *caddyfile.Dispenser, name string) (caddy.Duration, error) {
	val, err := parseStringArg(d, name)
	if err != nil {
		return 0, err
	}
	parsed, err := caddy.ParseDuration(val)
	if err != nil {
		return 0, d.Errf("%s must be a valid duration: %v", name, err)
	}
	return caddy.Duration(parsed), nil
}
