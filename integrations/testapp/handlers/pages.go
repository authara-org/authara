package handlers

import (
	"fmt"
	"html"
	"net/http"

	"github.com/authara-org/authara-go/authara"
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
	client := authara.NewClient("http://authara:8080") // Authara base URL

	user, err := client.GetCurrentUser(r.Context(), r)
	if err != nil {
		http.Error(w, "internal error aa", http.StatusInternalServerError)
		return
	}

	if user == nil {
		// http.Redirect(w, r, "/auth/login?return_to=/private", http.StatusSeeOther)
		return
	}

	logout, ok := authara.LogoutFormDataFromRequest(
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
				<a href="/auth/account">Show Account</a>
			</body>
			<script>
				window.addEventListener("pageshow", (event) => {
				  if (event.persisted) {
				    window.location.reload();
				  }
				});
			</script>
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
