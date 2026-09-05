package operator

func navAriaCurrent(active bool) string {
	if active {
		return "page"
	}
	return "false"
}

func navClass(active bool) string {
	base := "rounded-lg border px-3 py-2 font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 dark:focus:ring-offset-grey-950"
	if active {
		return base + " border-blue-600 bg-blue-600 text-white"
	}
	return base + " border-grey-200 bg-white text-grey-700 hover:border-grey-300 hover:bg-grey-50 dark:border-grey-800 dark:bg-grey-900 dark:text-grey-200 dark:hover:border-grey-700 dark:hover:bg-grey-800"
}
