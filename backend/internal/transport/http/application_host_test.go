package httptransport

import (
	"strings"
	"testing"
)

func TestApplicationHosts(t *testing.T) {
	const base = "remote.test"
	const id = "abcdef123456"
	for _, tc := range []struct {
		host            string
		reserved, valid bool
	}{
		{id + ".apps." + base, true, true},
		{id + ".apps." + base + ":8443", true, true},
		{"ABCDEF123456.apps." + base, true, true},
		{id + ".apps." + base + ".", true, true},
		{id + ".apps." + base + ".:8443", true, true},
		{"code." + id + ".apps." + base, true, false},
		{"code.extra." + id + ".apps." + base, true, false},
		{"-bad." + id + ".apps." + base, true, false},
		{"bad.apps." + base, true, false},
		{"apps." + base, true, false},
		{id + ".nested.apps." + base, true, false},
		{id + ".apps." + base + ".evil.test", false, false},
		{id + ".apps.other.test", false, false},
		{base, false, false},
	} {
		got, valid := ApplicationInstanceID(tc.host, base)
		if valid != tc.valid || (valid && got != id) || IsApplicationHost(tc.host, base) != tc.reserved {
			t.Errorf("host %q: id=%q valid=%v", tc.host, got, valid)
		}
	}
	if ApplicationHost(id, "", base) != id+".apps."+base || ApplicationHost("../bad", "", base) != "" {
		t.Fatal("incorrect application hostname construction")
	}
}

func TestProjectApplicationHosts(t *testing.T) {
	for _, label := range []string{"code", "editor-2"} {
		for _, slug := range []string{"gamerhead", "another-project"} {
			host := ApplicationHost(slug, label, "remote.test")
			if host != label+"--"+slug+".remote.test" {
				t.Fatal(host)
			}
			gotLabel, gotSlug, ok := ApplicationProject(host+":8443", "remote.test")
			if !ok || gotLabel != label || gotSlug != slug {
				t.Fatal(host, gotLabel, gotSlug)
			}
		}
	}
	for _, host := range []string{"code--project.remote.test.evil", "extra.code--project.remote.test", "code.-bad.remote.test", "code.project-.remote.test", "code..remote.test", "slug--3000.dev.remote.test", "slug.code.remote.test"} {
		if _, _, ok := ApplicationProject(host, "remote.test"); ok {
			t.Fatal(host)
		}
	}
}

func TestApplicationHostSeparatorAndLength(t *testing.T) {
	for _, slug := range []string{"code", "dev", "apps", "gamerhead", "other-project"} {
		host := ApplicationHost(slug, "code", "example.com")
		label, gotSlug, ok := ApplicationProject(strings.ToUpper(host)+".:443", "example.com")
		if !ok || label != "code" || gotSlug != slug {
			t.Fatal(host, label, gotSlug, ok)
		}
	}
	for _, host := range []string{"code.project.example.com", "code--a--b.example.com", "code---a.example.com", "--project.example.com", "code--.example.com", strings.Repeat("a", 58) + "--proj.example.com"} {
		if _, _, ok := ApplicationProject(host, "example.com"); ok {
			t.Fatal("accepted", host)
		}
	}
	if ApplicationHost("a--b", "code", "example.com") != "" {
		t.Fatal("accepted reserved separator")
	}
	if ApplicationHost("proj", strings.Repeat("a", 58), "example.com") != "" {
		t.Fatal("accepted oversized DNS label")
	}
	if ApplicationHost("proj", strings.Repeat("a", 57), "example.com") == "" {
		t.Fatal("rejected 63-character label")
	}
}
