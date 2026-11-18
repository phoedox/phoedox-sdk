package pdk

import (
	"strings"
	"testing"
)

func TestCheckSVG(t *testing.T) {
	ok := []string{
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M0 0h24v24H0z" fill="#41bdf5"/></svg>`,
		`<?xml version="1.0"?><!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd"><!-- by the brand --><svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><defs><linearGradient id="g"/></defs><style>.a{fill:url(#g)}</style><use xlink:href="#a"/><rect class="a" style="fill:url( '#g' )"/></svg>`,
	}
	for _, s := range ok {
		if err := CheckSVG([]byte(s)); err != nil {
			t.Errorf("%v: %s", err, s)
		}
	}
	bad := map[string]string{
		"script":        `<svg><script>alert(1)</script></svg>`,
		"handler":       `<svg onload="alert(1)"/>`,
		"external href": `<svg><use href="https://evil.example/x.svg#a"/></svg>`,
		"js href":       `<svg><a xlink:href="javascript:alert(1)"><rect/></a></svg>`,
		"image":         `<svg><image href="#x"/></svg>`,
		"foreign":       `<svg><foreignObject><div/></foreignObject></svg>`,
		"css url":       `<svg><style>rect{fill:url(https://evil.example/a)}</style></svg>`,
		"css import":    `<svg><style>@import "x.css";</style></svg>`,
		"attr url":      `<svg><rect style="background:url(//evil.example/a)"/></svg>`,
		"entity":        `<!DOCTYPE svg [<!ENTITY a "b">]><svg>&a;</svg>`,
		"not svg":       `<html><svg/></html>`,
		"empty":         ``,
		"animate":       `<svg><set attributeName="href" to="javascript:alert(1)"/></svg>`,
		"too big":       `<svg>` + strings.Repeat(" ", MaxLogoBytes) + `</svg>`,
	}
	for name, s := range bad {
		if err := CheckSVG([]byte(s)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestManifestCategoryLogo(t *testing.T) {
	base := `{"id":"de.example.x","name":"X","version":"1.0.0","hostApi":"1","engines":{"phoedox":">=1.0.0"}`
	for _, s := range []string{`,"category":"home","logo":"logo.svg","logoDark":"logo-dark.svg"}`, `}`, `,"category":"other"}`} {
		if _, err := ParseManifest([]byte(base + s)); err != nil {
			t.Errorf("%s: %v", s, err)
		}
	}
	for _, s := range []string{`,"category":"toys"}`, `,"logo":"img/logo.svg"}`, `,"logo":"logo.png"}`, `,"logo":"../logo.svg"}`, `,"logoDark":"d.svg"}`} {
		if _, err := ParseManifest([]byte(base + s)); err == nil {
			t.Errorf("%s: accepted", s)
		}
	}
}
