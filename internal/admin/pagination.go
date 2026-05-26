package admin

func normalizePage(page Page, defaultSize int) Page {
	if page.Page <= 0 {
		page.Page = 1
	}
	if page.Size <= 0 {
		page.Size = defaultSize
	}
	if page.Size > 100 {
		page.Size = 100
	}
	return page
}
