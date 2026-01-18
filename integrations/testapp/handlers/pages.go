package handlers

import (
	"fmt"
	"net/http"
)

func Home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, `
		<html>
			<body>
				<h1>TestApp</h1>
				<a href="/auth/login">Login</a>
			</body>
		</html>
	`)
}

func Private(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, `
		<html>
			<body>
				<h1>Private Page</h1>
				<p>You are authenticated.</p>
				<form method="POST" action="/auth/logout">
					<button type="submit">Logout</button>
				</form>
			</body>
		</html>
	`)
}
