package urlextractor

import (
	"testing"
)

// TestCategorizeURLs_Home tests home page categorization
func TestCategorizeURLs_Home(t *testing.T) {
	urls := []string{
		"https://example.com/",
		"https://example.com/home",
		"https://example.com/index.html",
		"https://example.it/homepage",
		"https://example.es/inicio",
		"https://example.fr/accueil",
		"https://example.de/startseite",
	}

	categories := CategorizeURLs(urls)

	if len(categories["home"]) != 7 {
		t.Errorf("Expected 7 home URLs, got %d", len(categories["home"]))
	}
}

// TestCategorizeURLs_Contact tests contact page categorization in multiple languages
func TestCategorizeURLs_Contact(t *testing.T) {
	urls := []string{
		"https://example.it/contatti",
		"https://example.it/contattaci",
		"https://example.com/contact",
		"https://example.com/contact-us",
		"https://example.es/contacto",
		"https://example.fr/contactez-nous",
		"https://example.de/kontakt",
	}

	categories := CategorizeURLs(urls)

	contactURLs := categories["contact"]
	if len(contactURLs) != 7 {
		t.Errorf("Expected 7 contact URLs, got %d", len(contactURLs))
	}
}

// TestCategorizeURLs_About tests about page categorization
func TestCategorizeURLs_About(t *testing.T) {
	urls := []string{
		"https://example.it/chi-siamo",
		"https://example.it/azienda",
		"https://example.com/about",
		"https://example.com/about-us",
		"https://example.com/who-we-are",
		"https://example.es/nosotros",
		"https://example.es/quienes-somos",
		"https://example.fr/a-propos",
		"https://example.de/uber-uns",
	}

	categories := CategorizeURLs(urls)

	aboutURLs := categories["about"]
	if len(aboutURLs) != 9 {
		t.Errorf("Expected 9 about URLs, got %d", len(aboutURLs))
	}
}

// TestCategorizeURLs_CaseInsensitive tests case-insensitive matching
func TestCategorizeURLs_CaseInsensitive(t *testing.T) {
	urls := []string{
		"https://example.com/CONTACT",
		"https://example.com/Contact",
		"https://example.com/contact",
		"https://example.com/CoNtAcT",
	}

	categories := CategorizeURLs(urls)

	contactURLs := categories["contact"]
	if len(contactURLs) != 4 {
		t.Errorf("Expected 4 contact URLs (case-insensitive), got %d", len(contactURLs))
	}
}

// TestCategorizeURLs_PathSegments tests matching in path segments
func TestCategorizeURLs_PathSegments(t *testing.T) {
	urls := []string{
		"https://example.com/en/contact",
		"https://example.com/it/contatti",
		"https://example.com/about/team",
		"https://example.com/en/about-us",
		"https://example.com/products/catalog",
		"https://example.com/fr/produits",
	}

	categories := CategorizeURLs(urls)

	// Should match contact pattern in /en/contact and /it/contatti
	if len(categories["contact"]) != 2 {
		t.Errorf("Expected 2 contact URLs in path segments, got %d", len(categories["contact"]))
	}

	// Should match about pattern
	if len(categories["about"]) != 2 {
		t.Errorf("Expected 2 about URLs in path segments, got %d", len(categories["about"]))
	}

	// Should match products pattern
	if len(categories["products"]) != 2 {
		t.Errorf("Expected 2 products URLs in path segments, got %d", len(categories["products"]))
	}
}

// TestCategorizeURLs_MultipleCategories tests URLs that match multiple categories
func TestCategorizeURLs_MultipleCategories(t *testing.T) {
	urls := []string{
		"https://example.com/blog",
		"https://example.com/news",
		"https://example.com/faq",
		"https://example.com/help",
	}

	categories := CategorizeURLs(urls)

	// blog and news should both be in "blog" category
	blogURLs := categories["blog"]
	if len(blogURLs) != 2 {
		t.Errorf("Expected 2 blog URLs, got %d", len(blogURLs))
	}

	// faq and help should both be in "faq" category
	faqURLs := categories["faq"]
	if len(faqURLs) != 2 {
		t.Errorf("Expected 2 faq URLs, got %d", len(faqURLs))
	}
}

// TestCategorizeURLs_AllCategories tests all 9 categories
func TestCategorizeURLs_AllCategories(t *testing.T) {
	urls := []string{
		"https://example.com/",          // home
		"https://example.com/contatti",  // contact
		"https://example.com/chi-siamo", // about
		"https://example.com/prodotti",  // products
		"https://example.com/blog",      // blog
		"https://example.com/faq",       // faq
		"https://example.com/privacy",   // privacy
		"https://example.com/login",     // login
		"https://example.com/carrello",  // cart
	}

	categories := CategorizeURLs(urls)

	expectedCategories := []string{
		"home", "contact", "about", "products", "blog", "faq", "privacy", "login", "cart",
	}

	for _, cat := range expectedCategories {
		if len(categories[cat]) != 1 {
			t.Errorf("Expected 1 URL for category %s, got %d", cat, len(categories[cat]))
		}
	}

	if len(categories) != 9 {
		t.Errorf("Expected 9 categories, got %d", len(categories))
	}
}

// TestCategorizeURLs_EmptyInput tests empty URL list
func TestCategorizeURLs_EmptyInput(t *testing.T) {
	urls := []string{}
	categories := CategorizeURLs(urls)

	if len(categories) != 0 {
		t.Errorf("Expected empty categories for empty input, got %d categories", len(categories))
	}
}

// TestCategorizeURLs_NoMatches tests URLs that don't match any category
func TestCategorizeURLs_NoMatches(t *testing.T) {
	urls := []string{
		"https://example.com/random-page",
		"https://example.com/something-else",
		"https://example.com/xyz123",
	}

	categories := CategorizeURLs(urls)

	if len(categories) != 0 {
		t.Errorf("Expected no categories for non-matching URLs, got %d categories", len(categories))
	}
}

// TestCategorizeURLs_InvalidURLs tests handling of invalid URLs
func TestCategorizeURLs_InvalidURLs(t *testing.T) {
	urls := []string{
		"https://example.com/contact",
		"not-a-valid-url",
		"://invalid",
		"https://example.com/about",
	}

	categories := CategorizeURLs(urls)

	// Should have contact and about, but skip invalid URLs
	if len(categories["contact"]) != 1 {
		t.Errorf("Expected 1 contact URL, got %d", len(categories["contact"]))
	}
	if len(categories["about"]) != 1 {
		t.Errorf("Expected 1 about URL, got %d", len(categories["about"]))
	}
}

// TestCategorizeURLs_Privacy tests privacy/legal categorization
func TestCategorizeURLs_Privacy(t *testing.T) {
	urls := []string{
		"https://example.it/privacy",
		"https://example.it/cookie",
		"https://example.com/terms",
		"https://example.com/privacy-policy",
		"https://example.de/datenschutz",
		"https://example.de/impressum",
		"https://example.fr/mentions-legales",
	}

	categories := CategorizeURLs(urls)

	privacyURLs := categories["privacy"]
	if len(privacyURLs) != 7 {
		t.Errorf("Expected 7 privacy URLs, got %d", len(privacyURLs))
	}
}

// TestCategorizeURLs_Login tests login/account categorization
func TestCategorizeURLs_Login(t *testing.T) {
	urls := []string{
		"https://example.it/login",
		"https://example.it/accedi",
		"https://example.com/signin",
		"https://example.com/register",
		"https://example.es/acceso",
		"https://example.fr/connexion",
		"https://example.de/anmelden",
	}

	categories := CategorizeURLs(urls)

	loginURLs := categories["login"]
	if len(loginURLs) != 7 {
		t.Errorf("Expected 7 login URLs, got %d", len(loginURLs))
	}
}

// TestCategorizeURLs_Cart tests cart/checkout categorization
func TestCategorizeURLs_Cart(t *testing.T) {
	urls := []string{
		"https://example.it/carrello",
		"https://example.it/checkout",
		"https://example.com/cart",
		"https://example.com/basket",
		"https://example.es/carrito",
		"https://example.fr/panier",
		"https://example.de/warenkorb",
	}

	categories := CategorizeURLs(urls)

	cartURLs := categories["cart"]
	if len(cartURLs) != 7 {
		t.Errorf("Expected 7 cart URLs, got %d", len(cartURLs))
	}
}

// TestCategorizeURLs_Products tests products/services categorization
func TestCategorizeURLs_Products(t *testing.T) {
	urls := []string{
		"https://example.it/prodotti",
		"https://example.it/servizi",
		"https://example.com/products",
		"https://example.com/services",
		"https://example.com/shop",
		"https://example.es/productos",
		"https://example.fr/produits",
		"https://example.de/produkte",
	}

	categories := CategorizeURLs(urls)

	productURLs := categories["products"]
	if len(productURLs) != 8 {
		t.Errorf("Expected 8 product URLs, got %d", len(productURLs))
	}
}

// TestCategorizeURLs_Blog tests blog/news categorization
func TestCategorizeURLs_Blog(t *testing.T) {
	urls := []string{
		"https://example.it/blog",
		"https://example.it/notizie",
		"https://example.com/news",
		"https://example.com/articles",
		"https://example.es/noticias",
		"https://example.fr/actualites",
		"https://example.de/nachrichten",
	}

	categories := CategorizeURLs(urls)

	blogURLs := categories["blog"]
	if len(blogURLs) != 7 {
		t.Errorf("Expected 7 blog URLs, got %d", len(blogURLs))
	}
}

// TestCategorizeURLs_FAQ tests FAQ/support categorization
func TestCategorizeURLs_FAQ(t *testing.T) {
	urls := []string{
		"https://example.it/faq",
		"https://example.it/aiuto",
		"https://example.com/help",
		"https://example.com/support",
		"https://example.es/preguntas",
		"https://example.fr/aide",
		"https://example.de/hilfe",
	}

	categories := CategorizeURLs(urls)

	faqURLs := categories["faq"]
	if len(faqURLs) != 7 {
		t.Errorf("Expected 7 FAQ URLs, got %d", len(faqURLs))
	}
}

// TestGetStandardPagesSummary tests the summary generation
func TestGetStandardPagesSummary(t *testing.T) {
	categories := map[string][]string{
		"home":    {"https://example.com/"},
		"contact": {"https://example.com/contact", "https://example.com/contatti"},
		"about":   {"https://example.com/about"},
	}

	summary := GetStandardPagesSummary(categories)

	// Should mention all three categories
	if !containsString(summary, "home") {
		t.Error("Summary should mention 'home'")
	}
	if !containsString(summary, "contact") {
		t.Error("Summary should mention 'contact'")
	}
	if !containsString(summary, "about") {
		t.Error("Summary should mention 'about'")
	}
}

// TestGetStandardPagesSummary_Empty tests empty summary
func TestGetStandardPagesSummary_Empty(t *testing.T) {
	categories := map[string][]string{}
	summary := GetStandardPagesSummary(categories)

	expected := "No standard pages found"
	if summary != expected {
		t.Errorf("Expected '%s', got '%s'", expected, summary)
	}
}

// TestMatchesPattern tests the pattern matching logic
func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		path     string
		pattern  string
		category PageCategory
		want     bool
	}{
		// Exact matches
		{"/contact", "/contact", CategoryContact, true},
		{"/about", "/about", CategoryAbout, true},

		// Case handling (path is already lowercased in real usage)
		{"/contact", "/contact", CategoryContact, true},

		// Path segments
		{"/en/contact", "/contact", CategoryContact, true},
		{"/it/contatti", "/contatti", CategoryContact, true},
		{"/about/team", "/about", CategoryAbout, true},

		// Non-matches
		{"/contacto", "/contact", CategoryContact, false},
		{"/product", "/products", CategoryProducts, false},

		// Root special case
		{"/", "/", CategoryHome, true},
		{"", "/", CategoryHome, true},

		// Language prefix exact match only (for home category)
		{"/it", "/it", CategoryHome, true},
		{"/en", "/en", CategoryHome, true},
		{"/it/contatti", "/it", CategoryHome, false}, // Should NOT match - language prefix with more segments
		{"/en/contact", "/en", CategoryHome, false},  // Should NOT match - language prefix with more segments

		// Language prefix in other categories should match segments
		{"/it/contatti", "/contatti", CategoryContact, true},
		{"/en/contact", "/contact", CategoryContact, true},
	}

	for _, tt := range tests {
		got := matchesPattern(tt.path, tt.pattern, tt.category)
		if got != tt.want {
			t.Errorf("matchesPattern(%q, %q, %v) = %v, want %v", tt.path, tt.pattern, tt.category, got, tt.want)
		}
	}
}

// TestCategorizeURLs_LanguagePrefixes tests language prefix detection for home pages
func TestCategorizeURLs_LanguagePrefixes(t *testing.T) {
	urls := []string{
		"https://example.com/it",
		"https://example.com/en",
		"https://example.com/es",
		"https://example.com/fr",
		"https://example.com/de",
		"https://example.com/pt",
		"https://example.com/ru",
		"https://example.com/zh",
		"https://example.com/ja",
		"https://example.com/ar",
		"https://example.com/nl",
		"https://example.com/en-us",
		"https://example.com/en-gb",
		"https://example.com/pt-br",
		"https://example.com/es-mx",
		"https://example.com/zh-cn",
	}

	categories := CategorizeURLs(urls)

	if len(categories["home"]) != 16 {
		t.Errorf("Expected 16 home URLs with language prefixes, got %d. URLs: %v", len(categories["home"]), categories["home"])
	}
}

// TestCategorizeURLs_FileExtensions tests that file extensions are properly handled
func TestCategorizeURLs_FileExtensions(t *testing.T) {
	urls := []string{
		"https://example.com/contact.html",
		"https://example.com/contact.htm",
		"https://example.com/contatti.php",
		"https://example.com/about.aspx",
		"https://example.com/products.jsp",
		"https://example.com/blog.cfm",
		"https://example.com/faq.shtml",
	}

	categories := CategorizeURLs(urls)

	if len(categories["contact"]) != 3 {
		t.Errorf("Expected 3 contact URLs (with extensions), got %d. URLs: %v", len(categories["contact"]), categories["contact"])
	}

	if len(categories["about"]) != 1 {
		t.Errorf("Expected 1 about URL (with .aspx), got %d", len(categories["about"]))
	}

	if len(categories["products"]) != 1 {
		t.Errorf("Expected 1 products URL (with .jsp), got %d", len(categories["products"]))
	}

	if len(categories["blog"]) != 1 {
		t.Errorf("Expected 1 blog URL (with .cfm), got %d", len(categories["blog"]))
	}

	if len(categories["faq"]) != 1 {
		t.Errorf("Expected 1 faq URL (with .shtml), got %d", len(categories["faq"]))
	}
}

// TestCategorizeURLs_TrailingSlashes tests that trailing slashes are normalized
func TestCategorizeURLs_TrailingSlashes(t *testing.T) {
	urls := []string{
		"https://example.com/contact/",
		"https://example.com/about/",
		"https://example.com/products/",
		"https://example.com/blog/",
		"https://example.com/",
	}

	categories := CategorizeURLs(urls)

	if len(categories["contact"]) != 1 {
		t.Errorf("Expected 1 contact URL (with trailing slash), got %d", len(categories["contact"]))
	}

	if len(categories["about"]) != 1 {
		t.Errorf("Expected 1 about URL (with trailing slash), got %d", len(categories["about"]))
	}

	if len(categories["products"]) != 1 {
		t.Errorf("Expected 1 products URL (with trailing slash), got %d", len(categories["products"]))
	}

	if len(categories["blog"]) != 1 {
		t.Errorf("Expected 1 blog URL (with trailing slash), got %d", len(categories["blog"]))
	}

	if len(categories["home"]) != 1 {
		t.Errorf("Expected 1 home URL (root with trailing slash), got %d", len(categories["home"]))
	}
}

// TestCategorizeURLs_EmptySegments tests handling of malformed URLs with double slashes
func TestCategorizeURLs_EmptySegments(t *testing.T) {
	urls := []string{
		"https://example.com//contact",
		"https://example.com//about//us",
		"https://example.com///products",
	}

	categories := CategorizeURLs(urls)

	// Should still match despite double slashes
	if len(categories["contact"]) != 1 {
		t.Errorf("Expected 1 contact URL (with double slash), got %d", len(categories["contact"]))
	}

	if len(categories["about"]) != 1 {
		t.Errorf("Expected 1 about URL (with double slashes), got %d", len(categories["about"]))
	}

	if len(categories["products"]) != 1 {
		t.Errorf("Expected 1 products URL (with triple slashes), got %d", len(categories["products"]))
	}
}

// TestCategorizeURLs_URLDecoding tests URL-encoded characters are properly decoded
func TestCategorizeURLs_URLDecoding(t *testing.T) {
	urls := []string{
		"https://example.de/uber-uns",            // uber-uns (German about, already decoded)
		"https://example.com/caf%C3%A9",          // café (encoded, won't match patterns but tests decoding)
		"https://example.com/contact%20us",       // contact us (encoded space)
		"https://example.com/%E8%81%94%E7%B3%BB", // 联系 (Chinese contact, encoded)
	}

	categories := CategorizeURLs(urls)

	// uber-uns should match about pattern
	if len(categories["about"]) != 1 {
		t.Errorf("Expected 1 about URL (uber-uns), got %d. URLs: %v", len(categories["about"]), categories["about"])
	}

	// contact with encoded space should match
	if len(categories["contact"]) < 1 {
		t.Errorf("Expected at least 1 contact URL (with encoded space), got %d. URLs: %v", len(categories["contact"]), categories["contact"])
	}
}

// TestCategorizeURLs_Portuguese tests Portuguese language patterns
func TestCategorizeURLs_Portuguese(t *testing.T) {
	urls := []string{
		"https://example.com.br/pt-br",
		"https://example.pt/pt-pt",
		"https://example.com/contato",
		"https://example.com/sobre-nos",
		"https://example.com/produtos",
		"https://example.com/artigos",
		"https://example.com/ajuda",
		"https://example.com/privacidade",
		"https://example.com/entrar",
		"https://example.com/carrinho",
	}

	categories := CategorizeURLs(urls)

	expectedCats := map[string]int{
		"home":     2, // pt-br, pt-pt
		"contact":  1, // contato
		"about":    1, // sobre-nos
		"products": 1, // produtos
		"blog":     1, // artigos
		"faq":      1, // ajuda
		"privacy":  1, // privacidade
		"login":    1, // entrar
		"cart":     1, // carrinho
	}

	for cat, expectedCount := range expectedCats {
		actualCount := len(categories[cat])
		if actualCount != expectedCount {
			t.Errorf("Portuguese - Category %s: expected %d URLs, got %d. URLs: %v",
				cat, expectedCount, actualCount, categories[cat])
		}
	}
}

// TestCategorizeURLs_Russian tests Russian language patterns
func TestCategorizeURLs_Russian(t *testing.T) {
	urls := []string{
		"https://example.ru/ru",
		"https://example.ru/kontakty",
		"https://example.ru/o-nas",
		"https://example.ru/produkty",
		"https://example.ru/novosti",
		"https://example.ru/pomoshch",
		"https://example.ru/konfidentsialnost",
		"https://example.ru/vkhod",
		"https://example.ru/korzina",
	}

	categories := CategorizeURLs(urls)

	expectedCats := map[string]int{
		"home":     1, // ru
		"contact":  1, // kontakty
		"about":    1, // o-nas
		"products": 1, // produkty
		"blog":     1, // novosti
		"faq":      1, // pomoshch
		"privacy":  1, // konfidentsialnost
		"login":    1, // vkhod
		"cart":     1, // korzina
	}

	for cat, expectedCount := range expectedCats {
		actualCount := len(categories[cat])
		if actualCount != expectedCount {
			t.Errorf("Russian - Category %s: expected %d URLs, got %d. URLs: %v",
				cat, expectedCount, actualCount, categories[cat])
		}
	}
}

// TestCategorizeURLs_Chinese tests Chinese language patterns
func TestCategorizeURLs_Chinese(t *testing.T) {
	urls := []string{
		"https://example.cn/zh",
		"https://example.cn/zh-cn",
		"https://example.tw/zh-tw",
		"https://example.cn/lianxi",
		"https://example.cn/guanyu",
		"https://example.cn/chanpin",
		"https://example.cn/xinwen",
		"https://example.cn/bangzhu",
		"https://example.cn/yinsi",
		"https://example.cn/denglu",
		"https://example.cn/gouwuche",
	}

	categories := CategorizeURLs(urls)

	expectedCats := map[string]int{
		"home":     3, // zh, zh-cn, zh-tw
		"contact":  1, // lianxi
		"about":    1, // guanyu
		"products": 1, // chanpin
		"blog":     1, // xinwen
		"faq":      1, // bangzhu
		"privacy":  1, // yinsi
		"login":    1, // denglu
		"cart":     1, // gouwuche
	}

	for cat, expectedCount := range expectedCats {
		actualCount := len(categories[cat])
		if actualCount != expectedCount {
			t.Errorf("Chinese - Category %s: expected %d URLs, got %d. URLs: %v",
				cat, expectedCount, actualCount, categories[cat])
		}
	}
}

// TestCategorizeURLs_Japanese tests Japanese language patterns
func TestCategorizeURLs_Japanese(t *testing.T) {
	urls := []string{
		"https://example.jp/ja",
		"https://example.jp/otoiawase",
		"https://example.jp/kaishagaiyou",
		"https://example.jp/seihin",
		"https://example.jp/nyusu",
		"https://example.jp/tasukeru",
		"https://example.jp/puraibashi",
		"https://example.jp/roguin",
		"https://example.jp/kaato",
	}

	categories := CategorizeURLs(urls)

	expectedCats := map[string]int{
		"home":     1, // ja
		"contact":  1, // otoiawase
		"about":    1, // kaishagaiyou
		"products": 1, // seihin
		"blog":     1, // nyusu
		"faq":      1, // tasukeru
		"privacy":  1, // puraibashi
		"login":    1, // roguin
		"cart":     1, // kaato
	}

	for cat, expectedCount := range expectedCats {
		actualCount := len(categories[cat])
		if actualCount != expectedCount {
			t.Errorf("Japanese - Category %s: expected %d URLs, got %d. URLs: %v",
				cat, expectedCount, actualCount, categories[cat])
		}
	}
}

// TestCategorizeURLs_Arabic tests Arabic language patterns
func TestCategorizeURLs_Arabic(t *testing.T) {
	urls := []string{
		"https://example.ae/ar",
		"https://example.ae/ittisal",
		"https://example.ae/anna",
		"https://example.ae/muntajat",
		"https://example.ae/akhbar",
		"https://example.ae/musaada",
		"https://example.ae/khususiya",
		"https://example.ae/dukhuul",
		"https://example.ae/sabt",
	}

	categories := CategorizeURLs(urls)

	expectedCats := map[string]int{
		"home":     1, // ar
		"contact":  1, // ittisal
		"about":    1, // anna
		"products": 1, // muntajat
		"blog":     1, // akhbar
		"faq":      1, // musaada
		"privacy":  1, // khususiya
		"login":    1, // dukhuul
		"cart":     1, // sabt
	}

	for cat, expectedCount := range expectedCats {
		actualCount := len(categories[cat])
		if actualCount != expectedCount {
			t.Errorf("Arabic - Category %s: expected %d URLs, got %d. URLs: %v",
				cat, expectedCount, actualCount, categories[cat])
		}
	}
}

// TestCategorizeURLs_Dutch tests Dutch language patterns
func TestCategorizeURLs_Dutch(t *testing.T) {
	urls := []string{
		"https://example.nl/nl",
		"https://example.nl/contactpagina",
		"https://example.nl/over-ons",
		"https://example.nl/producten",
		"https://example.nl/nieuws",
		"https://example.nl/hulp",
		"https://example.nl/privacy-nl",
		"https://example.nl/inloggen",
		"https://example.nl/winkelwagen",
	}

	categories := CategorizeURLs(urls)

	expectedCats := map[string]int{
		"home":     1, // nl
		"contact":  1, // contactpagina
		"about":    1, // over-ons
		"products": 1, // producten
		"blog":     1, // nieuws
		"faq":      1, // hulp
		"privacy":  1, // privacy-nl
		"login":    1, // inloggen
		"cart":     1, // winkelwagen
	}

	for cat, expectedCount := range expectedCats {
		actualCount := len(categories[cat])
		if actualCount != expectedCount {
			t.Errorf("Dutch - Category %s: expected %d URLs, got %d. URLs: %v",
				cat, expectedCount, actualCount, categories[cat])
		}
	}
}

// TestCategorizeURLs_RegionalVariants tests regional language variants
func TestCategorizeURLs_RegionalVariants(t *testing.T) {
	urls := []string{
		"https://example.com/en-us",
		"https://example.com/en-gb",
		"https://example.com/en-ca",
		"https://example.com/es-es",
		"https://example.com/es-mx",
		"https://example.com/fr-fr",
		"https://example.com/fr-ca",
		"https://example.com/pt-br",
		"https://example.com/pt-pt",
		"https://example.com/de-de",
		"https://example.com/de-at",
	}

	categories := CategorizeURLs(urls)

	if len(categories["home"]) != 11 {
		t.Errorf("Expected 11 home URLs with regional variants, got %d. URLs: %v", len(categories["home"]), categories["home"])
	}
}

// TestCategorizeURLs_MixedLanguagesRealWorld tests a realistic multilingual site
func TestCategorizeURLs_MixedLanguagesRealWorld(t *testing.T) {
	urls := []string{
		"https://www.example.com/",
		"https://www.example.com/it/",
		"https://www.example.com/en/",
		"https://www.example.com/it/contatti",
		"https://www.example.com/en/contact",
		"https://www.example.com/de/kontakt",
		"https://www.example.com/fr/contactez-nous",
		"https://www.example.com/pt/contato",
		"https://www.example.com/it/chi-siamo.html",
		"https://www.example.com/en/about-us.php",
		"https://www.example.com/it/prodotti/",
		"https://www.example.com/en/products/",
		"https://www.example.com/blog",
		"https://www.example.com/it/privacy",
		"https://www.example.com/login.aspx",
	}

	categories := CategorizeURLs(urls)

	expectedCats := map[string]int{
		"home":     3, // /, /it/, /en/
		"contact":  5, // IT, EN, DE, FR, PT
		"about":    2, // IT (.html), EN (.php)
		"products": 2, // IT (trailing slash), EN (trailing slash)
		"blog":     1, // /blog
		"privacy":  1, // IT
		"login":    1, // .aspx
	}

	for cat, expectedCount := range expectedCats {
		actualCount := len(categories[cat])
		if actualCount != expectedCount {
			t.Errorf("Mixed real-world - Category %s: expected %d URLs, got %d. URLs: %v",
				cat, expectedCount, actualCount, categories[cat])
		}
	}
}

// TestStripFileExtension tests the stripFileExtension helper function
func TestStripFileExtension(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/contact.html", "/contact"},
		{"/contact.htm", "/contact"},
		{"/about.php", "/about"},
		{"/products.aspx", "/products"},
		{"/cart.jsp", "/cart"},
		{"/blog.cfm", "/blog"},
		{"/faq.shtml", "/faq"},
		{"/contact", "/contact"},         // no extension
		{"/contact.pdf", "/contact.pdf"}, // not a web extension
		{"/about.action", "/about"},
		{"/search.do", "/search"},
	}

	for _, tt := range tests {
		result := stripFileExtension(tt.input)
		if result != tt.expected {
			t.Errorf("stripFileExtension(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestFilterEmptySegments tests the filterEmptySegments helper function
func TestFilterEmptySegments(t *testing.T) {
	tests := []struct {
		input    []string
		expected []string
	}{
		{[]string{"en", "contact"}, []string{"en", "contact"}},
		{[]string{"", "contact"}, []string{"contact"}},
		{[]string{"en", "", "contact"}, []string{"en", "contact"}},
		{[]string{"", "", "contact"}, []string{"contact"}},
		{[]string{}, []string{}},
		{[]string{""}, []string{}},
		{[]string{"", "", ""}, []string{}},
		{[]string{"a", "b", "c"}, []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		result := filterEmptySegments(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("filterEmptySegments(%v) length = %d, want %d", tt.input, len(result), len(tt.expected))
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("filterEmptySegments(%v)[%d] = %q, want %q", tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}

// TestIsLanguagePrefix tests the language prefix regex
func TestIsLanguagePrefix(t *testing.T) {
	tests := []struct {
		pattern string
		want    bool
	}{
		// Valid language codes (2 letters)
		{"/it", true},
		{"/en", true},
		{"/es", true},
		{"/fr", true},
		{"/de", true},
		{"/pt", true},
		{"/ru", true},
		{"/zh", true},
		{"/ja", true},
		{"/ar", true},
		{"/nl", true},
		{"/ko", true},
		{"/pl", true},
		{"/vi", true},
		{"/th", true},

		// Valid regional variants (2-2 letters)
		{"/en-us", true},
		{"/en-gb", true},
		{"/en-ca", true},
		{"/pt-br", true},
		{"/pt-pt", true},
		{"/es-mx", true},
		{"/es-es", true},
		{"/fr-ca", true},
		{"/fr-fr", true},
		{"/zh-cn", true},
		{"/zh-tw", true},
		{"/de-de", true},
		{"/de-at", true},
		{"/de-ch", true},

		// Invalid - not language codes
		{"/contact", false},
		{"/about", false},
		{"/products", false},
		{"/blog", false},
		{"/home", false},

		// Invalid - wrong format
		{"/e", false},      // too short
		{"/eng", false},    // too long
		{"/en-", false},    // incomplete regional
		{"/en-u", false},   // incomplete regional
		{"/en-usa", false}, // regional part too long
		{"/EN", false},     // uppercase (pattern should be lowercase)
		{"/en-US", false},  // uppercase regional (pattern should be lowercase)
		{"en", false},      // missing leading slash
		{"/en/", false},    // trailing slash (normalized out before this check)
		{"/123", false},    // numbers
		{"/en-123", false}, // numbers in regional
		{"/e1", false},     // contains number
		{"/en-u1", false},  // number in regional part
	}

	for _, tt := range tests {
		got := isLanguagePrefix(tt.pattern)
		if got != tt.want {
			t.Errorf("isLanguagePrefix(%q) = %v, want %v", tt.pattern, got, tt.want)
		}
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr ||
			len(s) >= len(substr) && (s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				findInString(s, substr)))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
