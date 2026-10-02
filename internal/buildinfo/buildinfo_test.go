package buildinfo

import "testing"

func TestStableContract(t *testing.T) {
	for _, v := range []string{"0.0.0", "1.10.2", "999999999999999999999999.1.0"} {
		if got, err := FromTag("v" + v); err != nil || got != v {
			t.Fatalf("%s: %s %v", v, got, err)
		}
	}
	for _, tag := range []string{"1.2.3", "v01.2.3", "v1.2", "v1.2.3-rc.1", "v1.2.3+meta", "v1.2.3\n", "vv1.2.3"} {
		if _, err := FromTag(tag); err == nil {
			t.Fatalf("accepted %q", tag)
		}
	}
	for _, pair := range [][2]string{{"1.2.0", "1.10.0"}, {"1.9.9", "2.0.0"}, {"0.0.9", "0.0.10"}, {"9.0.0", "999999999999999999999999.0.0"}} {
		if Compare(pair[0], pair[1]) != -1 || Compare(pair[1], pair[0]) != 1 || Compare(pair[0], pair[0]) != 0 {
			t.Fatal(pair)
		}
	}
	for _, os := range []string{"darwin", "linux"} {
		for _, arch := range []string{"amd64", "arm64"} {
			a, m, err := AssetNames("1.2.3", os, arch)
			if err != nil || a != "wrk_1.2.3_"+os+"_"+arch+".tar.gz" || m != "wrk_1.2.3_checksums.txt" {
				t.Fatalf("%s %s %v", a, m, err)
			}
		}
	}
	if _, _, err := AssetNames("1.2.3", "windows", "amd64"); err == nil {
		t.Fatal("accepted Windows")
	}
	if (Info{Version: "1.2.3", BuildKind: "development"}).IsRelease() {
		t.Fatal("development appears to be release")
	}
}
