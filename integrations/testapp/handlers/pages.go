package handlers

import (
	"fmt"
	"html"
	"net/http"

	"github.com/alexlup06-authgate/authgate-go/authgate"
)

func Home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, `
		<html>
			<body>
				<h1>TestApp</h1>
				<a href="/auth/login?return_to=/private">Login</a>
			</body>
		</html>
	`)
}

func Private(w http.ResponseWriter, r *http.Request) {
	// Backend AuthGate client (usually created once and reused)
	client := authgate.NewClient("http://authgate:8080") // AuthGate base URL

	fmt.Println(authgate.UserIDFromContext(r.Context()))

	// Fetch current user identity
	user, err := client.GetCurrentUser(r.Context(), r)
	if err != nil {
		http.Error(w, "internal error aa", http.StatusInternalServerError)
		return
	}

	if user == nil {
		// http.Redirect(w, r, "/auth/login?return_to=/private", http.StatusSeeOther)
		return
	}

	logout, ok := authgate.LogoutFormDataFromRequest(
		r,
		"/auth/login?return_to=/private",
	)
	if !ok {
		http.Redirect(w, r, "/auth/login?return_to=/private", http.StatusSeeOther)
		return
	}

	fmt.Fprintf(w, `
		<html>
			<body>
				<h1>Private Page</h1>
				<p>You are authenticated.</p>
				<p><strong>Email:</strong> %s</p>
				<p><strong>Username:</strong> %s</p>

				<form method="%s" action="%s">
					<input type="hidden" name="%s" value="%s">
					<button type="submit">Logout</button>
				</form>
				<a href="/auth/user/account">Show Account</a>
			</body>
		</html>
	`,
		html.EscapeString(user.Email),
		html.EscapeString(user.Username),
		logout.Method,
		logout.Action,
		logout.CSRFName,
		logout.CSRFValue,
	)
}
