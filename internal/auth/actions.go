package auth

// Audit action names for authentication events. Defined here rather than
// in the audit package, since audit only knows how to store an event, not
// what any particular action means.
const (
	ActionLoginSucceeded = "user.login_succeeded"
	ActionLoginFailed    = "user.login_failed"
	ActionLogout         = "user.logout"
)
