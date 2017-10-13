package command

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"

	tlsIO "github.com/kelda/kelda/connection/tls/io"
	"github.com/kelda/kelda/util"
)

func TestParsePrivateKey(t *testing.T) {
	util.AppFs = afero.NewMemMapFs()

	// Test error on non-existent private key.
	_, err := parseSSHPrivateKey("dne")
	assert.EqualError(t, err, "read file: open dne: file does not exist")

	// Test error on malformed private key.
	err = util.WriteFile("malformed", []byte("malformed"), 0444)
	assert.NoError(t, err)
	_, err = parseSSHPrivateKey("malformed")
	assert.EqualError(t, err, "ssh: no key found")

	// Test we properly parse a legitimate key.
	pubKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC0XGWf3uoKJwIAh" +
		"g3jg1oNfyxav55IKDEq2W72CL2SRiMRtmVs6fPeaem6HNMvWFrb0pqnguQyHo59RT" +
		"4Hs/VJrbqkfR3wGWtxWL/TlVN0D/jSpOZP+/tuNz/qusow4PRlwvpV3Ic7JGNgcRP" +
		"vseR1mimM4PXAbqPfgay9OZ8WweaartZHT9iStSo64DYArDgJ3dV6M8RFqXPkbijT" +
		"8EfFuRj9PxH+S9NIzQF/T6rOemgMIBXDX9PA0DR/rGwYyaqHPMVsh+dw6Nsq/l21v" +
		"noURtc9U7AvrkL/42DKMnv/p16w6DZJuTqh/CzU29fI0PvfeTqhv0aF5mjYtkEo0Tb7"
	privKey := `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAtFxln97qCicCAIYN44NaDX8sWr+eSCgxKtlu9gi9kkYjEbZl
bOnz3mnpuhzTL1ha29Kap4LkMh6OfUU+B7P1Sa26pH0d8BlrcVi/05VTdA/40qTm
T/v7bjc/6rrKMOD0ZcL6VdyHOyRjYHET77HkdZopjOD1wG6j34GsvTmfFsHmmq7W
R0/YkrUqOuA2AKw4Cd3VejPERalz5G4o0/BHxbkY/T8R/kvTSM0Bf0+qznpoDCAV
w1/TwNA0f6xsGMmqhzzFbIfncOjbKv5dtb56FEbXPVOwL65C/+NgyjJ7/6desOg2
Sbk6ofws1NvXyND733k6ob9GheZo2LZBKNE2+wIDAQABAoIBAHow3uigrQ6Tvtd7
+ozYwHnEXthcWW+pSyYsiPBGm6gtvDSTzcMr/PwB5UchoDHDOksTM5OpKdCKwx47
evrdAKEaAgjOeynfDtuLtOozkIZhC8Ip1Z76qCzTYYo1YiYbQXhv0Am7jiKTVIBS
G5+YdZ73Ao9fGR911a/mupC4KP/Q2ogFaRlJ7JP6yUAPy5Kt5Vvgs1+og/D7lQF7
kWrgj85iO8sUePqhHbuRbAumsAWwe2LzzOGOllGvIrvp+Npxngk1m/usEXnW4Of4
49nut/IFFS0bVQGJCxYxeTnwcW3qAIr1tuY5586H0+S77uvhD3FDgsCdEQRQU8ul
4F9N1gECgYEA3ko5LPvmZTXtJYT3znPGrNXZ9Pt0J+bJUMK4aoMLkcfooanjIXp4
uNi8IArO4vhXt4STsLEtxAaLVhfGKdpkuqGzkcdpK4ezLF8ZMTpj/CdIjX6DcQ0i
R0w7z5DZpaSiYB4NtNgiQhCg7uxfCGBifCXvlsvCOZPET4JE/Y6lxMsCgYEAz7Zm
EHCqFexIirDIJDEksLGJF+/H/Cp6oA7jHWQSCZP9PfXQVqSzUf4jp/+yKRda3W/A
hK7GFLUp59gY4LUnofkqJugkGeHIBnFW2kTRTU33sljHkA7oiB02AhMw6zbKbFNB
csyLuj+3HHC+v91Qunnd19emapLrztIwbwXuQJECgYAJSWyOFpAPlmsr8BwyQeAB
BIYwl/jIWfn7J8dwm7z2ADYV2vUkRuuYPWXOqOTv0pRHlIBfF2fkEqnrlN6wjPE8
YtkPtBcOvIKdzfNNfTUEKdf8IVb4eCYAeIzfJRwSsYgfH+JOteDoha1TjgiCXxR+
P099K1IX+bZv4+9h8H24dQKBgQCBETLcllVp5/evjmfe7VaCIN8yK4HV9ENcP8Pq
WGtI3ldm7960aAUxNrzLQHxhQizpGe7Dw6I77dKLSOE0h/yHjj8eC/OazYwwTK8O
U+LGqWL3xGjE4C6nnZcYtPoZvmML6rPpdKaCZeMPXhN5PzlRljY+T7cN1BuI2VzV
MBc6sQKBgQDEsZz/tZ5SU6aNI8567r3n8Tw6aBeEfSEg6yA4DXN4bngH0qrD02Qc
DeDx0cPidMPsCf5E/7nkbp6ldLYVwsY3N4FyY7jmxzBDF9TP2Wgzt3vykkP1J5Wn
WEteRuQXq8oploci8N2U0C8zgKbH+fsKD6KeX/xI/EJ/8cktT0fLaA==
-----END RSA PRIVATE KEY-----`

	err = util.WriteFile("sshkey", []byte(privKey), 0444)
	assert.NoError(t, err)

	parsedPrivKey, err := parseSSHPrivateKey("sshkey")
	assert.NoError(t, err)
	assert.NotNil(t, parsedPrivKey)
	assert.Equal(t, pubKey, getPublicKey(parsedPrivKey))

	assert.Equal(t, "", getPublicKey(nil))
}

// Test that the generated files can be parsed.
func TestSetupTLS(t *testing.T) {
	util.AppFs = afero.NewMemMapFs()

	tlsDir := "tls"
	err := setupTLS(tlsDir)
	assert.NoError(t, err)

	_, err = tlsIO.ReadCredentials(tlsDir)
	assert.NoError(t, err)
}

// Test that the generated file can be parsed.
func TestSetupSSHKey(t *testing.T) {
	util.AppFs = afero.NewMemMapFs()

	keyPath := "ssh_key"
	err := setupSSHKey(keyPath)
	assert.NoError(t, err)

	_, err = parseSSHPrivateKey(keyPath)
	assert.NoError(t, err)
}
