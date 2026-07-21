package appcore

import "aw/internal/dto"

// CodesignTrustStatus inspects THIS app's code signature. A locally built app
// is signed with a self-signed identity; until macOS trusts that certificate
// for code signing, Keychain "Always Allow" grants never persist (securityd
// re-validates the signer chain on every access, fails, and re-prompts). The
// Security page uses this to offer the one-click fix below.
func (a *App) CodesignTrustStatus() dto.CodesignTrustResult {
	a.recordActivity()
	status, name := codesignTrustStatusNative()
	switch status {
	case 0:
		return dto.CodesignTrustResult{Supported: true, HasCertificate: true, Trusted: true, CertificateName: name}
	case 1:
		return dto.CodesignTrustResult{Supported: true, HasCertificate: true, Trusted: false, CertificateName: name}
	case 2:
		return dto.CodesignTrustResult{Supported: true, HasCertificate: false}
	default:
		return dto.CodesignTrustResult{Supported: false}
	}
}

// CodesignTrustGrant marks the app's signing certificate as trusted for code
// signing in the current user's trust settings. macOS itself asks the user to
// confirm with their account password — the app never sees that password.
func (a *App) CodesignTrustGrant() OperationResult {
	a.recordActivity()
	return a.basicOperationResult(codesignTrustGrantNative())
}
