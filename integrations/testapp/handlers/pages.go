package handlers

import (
	"fmt"
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
	logout, ok := authgate.LogoutFormDataFromRequest(r, "/auth/login?return_to=/private")
	if !ok {
		http.Redirect(w, r, "/auth/login?return_to=/private", http.StatusSeeOther)
		return
	}

	fmt.Fprintf(w, `
		<html>
			<body>
				<h1>Private Page</h1>
				<p>You are authenticated.</p>

				<form method="%s" action="%s">
					<input type="hidden" name="%s" value="%s">
					<button type="submit">Logout</button>
				</form>
			</body>
		</html>
	`,
		logout.Method,
		logout.Action,
		logout.CSRFName,
		logout.CSRFValue,
	)
}
