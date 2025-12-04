package urlextractor

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// PageCategory represents a type of standard web page
type PageCategory string

const (
	CategoryHome     PageCategory = "home"
	CategoryContact  PageCategory = "contact"
	CategoryAbout    PageCategory = "about"
	CategoryProducts PageCategory = "products"
	CategoryBlog     PageCategory = "blog"
	CategoryFAQ      PageCategory = "faq"
	CategoryPrivacy  PageCategory = "privacy"
	CategoryLogin    PageCategory = "login"
	CategoryCart     PageCategory = "cart"
)

// categoryPatterns defines URL patterns for standard page types in multiple languages
// Patterns are matched case-insensitively against the URL path
var categoryPatterns = map[PageCategory][]string{
	// Home page patterns
	CategoryHome: {
		"/", "/home", "/index", "/homepage", "/home-page", "/main", // generic
		// Language prefixes (common for multilingual sites)
		"/it", "/en", "/es", "/fr", "/de", "/pt", "/ru", "/zh", "/ja", "/ar", "/nl", // language codes
		"/en-us", "/en-gb", "/en-ca", "/en-au", // English regional
		"/es-es", "/es-mx", "/es-ar", "/es-co", // Spanish regional
		"/fr-fr", "/fr-ca", "/fr-be", // French regional
		"/pt-br", "/pt-pt", // Portuguese regional
		"/zh-cn", "/zh-tw", "/zh-hk", // Chinese regional
		"/de-de", "/de-at", "/de-ch", // German regional
		// Language-specific home pages
		"/inicio", "/portada", // Spanish
		"/accueil", "/accueil-fr", // French
		"/startseite", "/hauptseite", // German
		"/home-page", "/pagina-inicial", "/inicio-pt", // Portuguese
		"/glavnaya", "/domoj", // Russian (главная, домой transliterated)
		"/shouye", "/zhuye", // Chinese (首页, 主页 transliterated)
		"/homu", "/toppu", // Japanese (ホーム, トップ transliterated)
		"/home-ar", "/raisa", // Arabic
		"/startpagina", "/homepage-nl", // Dutch
	},

	// Contact page patterns
	CategoryContact: {
		"/contact", "/contact-us", "/contacts", "/contactus", "/get-in-touch", // English
		"/contatti", "/contattaci", "/contatto", // Italian
		"/contacto", "/contactanos", "/contactenos", // Spanish
		"/contactez-nous", "/nous-contacter", "/contact-fr", // French
		"/kontakt", "/kontaktieren", "/kontaktiere-uns", // German
		"/contato", "/contatos", "/fale-conosco", // Portuguese
		"/kontakt-ru", "/svyaz", "/kontakty", // Russian (связь, контакты transliterated)
		"/lianxi", "/联系", // Chinese (联系 transliterated)
		"/otoiawase", "/renraku", // Japanese (お問い合わせ, 連絡 transliterated)
		"/ittisal", "/contact-ar", // Arabic
		"/contacteer", "/contact-nl", "/contactpagina", // Dutch
	},

	// About page patterns
	CategoryAbout: {
		"/about", "/about-us", "/aboutus", "/company", "/who-we-are", "/our-story", "/team", // English
		"/chi-siamo", "/azienda", "/storia", "/la-nostra-storia", // Italian
		"/nosotros", "/quienes-somos", "/sobre-nosotros", "/acerca-de", // Spanish
		"/a-propos", "/qui-sommes-nous", "/notre-histoire", // French
		"/uber-uns", "/ueber-uns", "/unternehmen", "/firma", // German
		"/sobre-nos", "/quem-somos", "/empresa", "/nossa-historia", // Portuguese
		"/o-nas", "/o-kompanii", "/nasha-istoriya", // Russian (о нас, о компании, наша история transliterated)
		"/guanyu", "/guanyuwomen", "/关于", // Chinese (关于, 关于我们 transliterated)
		"/kaishagaiyou", "/wareware", // Japanese (会社概要, 我々 transliterated)
		"/anna", "/hawlana", "/about-ar", // Arabic (حولنا transliterated)
		"/over-ons", "/bedrijf", "/ons-verhaal", // Dutch
	},

	// Products/Services page patterns
	CategoryProducts: {
		"/products", "/services", "/catalog", "/shop", "/store", "/solutions", "/offerings", // English
		"/prodotti", "/servizi", "/catalogo", "/negozio", // Italian
		//nolint:misspell // Spanish words: productos, servicios, tienda, catalogo-es
		"/productos", "/servicios", "/tienda", "/catalogo-es", // Spanish
		//nolint:misspell // French word: catalogue
		"/produits", "/services-fr", "/boutique", "/catalogue", // French
		"/produkte", "/dienstleistungen", "/katalog", // German
		"/produtos", "/servicos", "/loja", "/catalogo-pt", // Portuguese
		"/produkty", "/uslugi", "/magazin", "/katalog-ru", // Russian (продукты, услуги, магазин transliterated)
		"/chanpin", "/fuwu", "/shangdian", // Chinese (产品, 服务, 商店 transliterated)
		"/seihin", "/sabisu", "/mise", // Japanese (製品, サービス, 店 transliterated)
		"/muntajat", "/khidmat", "/products-ar", // Arabic (منتجات, خدمات transliterated)
		"/producten", "/diensten", "/winkel", "/catalogus", // Dutch
	},

	// Blog/News page patterns
	CategoryBlog: {
		"/blog",                                                 // generic
		"/news", "/articles", "/insights", "/updates", "/press", // English
		"/notizie", "/articoli", "/novita", // Italian
		"/noticias", "/articulos", // Spanish
		"/actualites", "/nouvelles", "/articles-fr", // French
		"/nachrichten", "/neuigkeiten", "/artikel", // German
		"/noticias-pt", "/artigos", "/novidades", "/imprensa", // Portuguese
		"/novosti", "/stati", "/blog-ru", // Russian (новости, статьи transliterated)
		"/xinwen", "/wenzhang", "/boke", // Chinese (新闻, 文章, 博客 transliterated)
		"/nyusu", "/kiji", "/buroggu", // Japanese (ニュース, 記事, ブログ transliterated)
		"/akhbar", "/maqalat", "/blog-ar", // Arabic (أخبار, مقالات transliterated)
		"/nieuws", "/artikelen", "/blog-nl", // Dutch
	},

	// FAQ/Support page patterns
	CategoryFAQ: {
		"/faq", "/support", // generic
		"/help", "/questions", "/customer-service", "/helpdesk", // English
		"/domande", "/aiuto", "/supporto", "/assistenza", "/domande-frequenti", // Italian
		"/preguntas", "/ayuda", "/soporte", "/preguntas-frecuentes", // Spanish
		"/aide", "/questions-frequentes", // French
		"/hilfe", "/haeufige-fragen", // German
		"/ajuda", "/perguntas", "/perguntas-frequentes", "/suporte-pt", // Portuguese
		"/pomoshch", "/voprosy", "/podderzhka", // Russian (помощь, вопросы, поддержка transliterated)
		"/bangzhu", "/wenti", "/zhichi", // Chinese (帮助, 问题, 支持 transliterated)
		"/tasukeru", "/shitsumon", "/sapoto", // Japanese (助ける, 質問, サポート transliterated)
		"/musaada", "/asila", "/support-ar", // Arabic (مساعدة, أسئلة transliterated)
		"/hulp", "/vragen", "/veelgestelde-vragen", // Dutch
	},

	// Privacy/Legal page patterns
	CategoryPrivacy: {
		"/privacy", "/privacy-policy", // generic
		"/terms", "/legal", "/cookies", "/terms-of-service", "/terms-and-conditions", // English
		"/termini", "/cookie", "/informativa-privacy", "/termini-condizioni", // Italian
		"/privacidad", "/terminos", "/politica-privacidad", "/condiciones", // Spanish
		"/confidentialite", "/mentions-legales", "/politique-confidentialite", "/cgv", // French
		"/datenschutz", "/impressum", "/agb", "/rechtliches", "/nutzungsbedingungen", // German
		"/privacidade", "/termos", "/politica-privacidade", "/condicoes", // Portuguese
		"/konfidentsialnost", "/usloviya", "/politika-konfidentsialnosti", // Russian (конфиденциальность, условия transliterated)
		"/yinsi", "/tiaokuan", "/falv", // Chinese (隐私, 条款, 法律 transliterated)
		"/puraibashi", "/kiyaku", "/riyoukiyaku", // Japanese (プライバシー, 規約, 利用規約 transliterated)
		"/khususiya", "/shurut", "/privacy-ar", // Arabic (خصوصية, شروط transliterated)
		"/privacy-nl", "/voorwaarden", "/juridisch", // Dutch
	},

	// Login/Account page patterns
	CategoryLogin: {
		"/login",                                                                                   // generic
		"/signin", "/sign-in", "/register", "/signup", "/sign-up", "/my-account", "/user", "/auth", // English
		"/accedi", "/account", "/registrati", "/area-riservata", "/entra", // Italian
		"/acceso", "/ingresar", "/registro", "/iniciar-sesion", "/entrar", // Spanish
		//nolint:misspell // French word: connexion
		"/connexion", "/se-connecter", "/inscription", "/mon-compte", // French
		"/anmelden", "/registrieren", "/einloggen", "/konto", // German
		"/entrar", "/login-pt", "/registro-pt", "/minha-conta", // Portuguese
		"/vkhod", "/registratsiya", "/moj-akkaunt", // Russian (вход, регистрация, мой аккаунт transliterated)
		"/denglu", "/zhuce", "/wode-zhanghu", // Chinese (登录, 注册, 我的账户 transliterated)
		"/roguin", "/touroku", "/akaunts", // Japanese (ログイン, 登録, アカウント transliterated)
		"/dukhuul", "/tasjil", "/login-ar", // Arabic (دخول, تسجيل transliterated)
		"/inloggen", "/registreren", "/mijn-account", // Dutch
	},

	// Cart/Checkout page patterns
	CategoryCart: {
		"/cart", "/basket", "/checkout", "/order", "/shopping-cart", "/bag", // English
		"/carrello", "/ordine", "/cassa", "/acquista", // Italian
		"/carrito", "/cesta", "/pedido", "/comprar", // Spanish
		"/panier", "/commande", "/acheter", // French
		"/warenkorb", "/kasse", "/bestellen", "/einkaufswagen", // German
		"/carrinho", "/cesta-pt", "/pedido-pt", "/comprar-pt", "/finalizar", // Portuguese
		"/korzina", "/zakaz", "/oformit", // Russian (корзина, заказ, оформить transliterated)
		"/gouwuche", "/dingdan", "/jiesuan", // Chinese (购物车, 订单, 结算 transliterated)
		"/kaato", "/chuumon", "/kaikei", // Japanese (カート, 注文, 会計 transliterated)
		"/sabt", "/talabiya", "/cart-ar", // Arabic (سبت, طلبية transliterated)
		"/winkelwagen", "/bestelling", "/afrekenen", // Dutch
	},
}

// CategorizeURLs analyzes a list of URLs and categorizes them into standard page types.
// It returns a map keyed by PageCategory where each entry contains the URLs that matched that category.
//
// The categorization is:
// - Case-insensitive: "/Contact" matches "contact" pattern
// - Path-aware: "https://example.com/en/contact" matches "contact" pattern
// - Multi-match: A URL can appear in multiple categories if it matches multiple patterns
//
// Example:
//
//	urls := []string{
//	    "https://example.com/",
//	    "https://example.com/contatti",
//	    "https://example.com/en/about-us",
//	}
//	categories := CategorizeURLs(urls)
//	// Returns: map[PageCategory][]string{
//	//   CategoryHome:    {"https://example.com/"},
//	//   CategoryContact: {"https://example.com/contatti"},
//	//   CategoryAbout:   {"https://example.com/en/about-us"},
//	// }
func CategorizeURLs(urls []string) map[PageCategory][]string {
	categories := make(map[PageCategory][]string)

	for _, urlStr := range urls {
		// Parse URL to extract path
		parsedURL, err := url.Parse(urlStr)
		if err != nil {
			continue // Skip invalid URLs
		}

		// URL decode the path to handle encoded characters like %C3%BC (ü)
		decodedPath, err := url.QueryUnescape(parsedURL.Path)
		if err != nil {
			// If decoding fails, use original path
			decodedPath = parsedURL.Path
		}

		// Normalize path: lowercase and remove trailing slash (except for root)
		path := strings.ToLower(decodedPath)
		if path != "/" && strings.HasSuffix(path, "/") {
			path = strings.TrimSuffix(path, "/")
		}

		// Remove common file extensions to match patterns
		path = stripFileExtension(path)

		// Check against all category patterns
		for category, patterns := range categoryPatterns {
			if matchesAnyPattern(path, patterns, category) {
				categories[category] = append(categories[category], urlStr)
			}
		}
	}

	return categories
}

// stripFileExtension removes common file extensions from a path
// This allows /contact.html to match /contact pattern
func stripFileExtension(path string) string {
	extensions := []string{
		".html", ".htm", ".php", ".asp", ".aspx", ".jsp",
		".do", ".action", ".cfm", ".pl", ".cgi", ".shtml",
	}

	for _, ext := range extensions {
		if strings.HasSuffix(path, ext) {
			return strings.TrimSuffix(path, ext)
		}
	}

	return path
}

// matchesAnyPattern checks if a path matches any of the given patterns.
// Patterns can match:
// - Exact path: "/contact" matches "/contact"
// - Path segment: "/contact" pattern matches "/en/contact", "/it/contatti/form"
// - Root variations: "/" matches "" or "/"
// - Language prefixes (for home category): "/it" only matches "/it" exactly, not "/it/something"
func matchesAnyPattern(path string, patterns []string, category PageCategory) bool {
	for _, pattern := range patterns {
		if matchesPattern(path, pattern, category) {
			return true
		}
	}
	return false
}

// matchesPattern checks if a path matches a specific pattern.
// Matching rules:
// - Exact match: path == pattern
// - Path contains pattern as a complete segment (not substring)
// - Special handling for language prefix patterns in home category (must be exact match only)
func matchesPattern(path, pattern string, category PageCategory) bool {
	// Normalize pattern: lowercase and remove trailing slash (except for root)
	pattern = strings.ToLower(pattern)
	if pattern != "/" && strings.HasSuffix(pattern, "/") {
		pattern = strings.TrimSuffix(pattern, "/")
	}

	// Special case: root path
	if pattern == "/" {
		return path == "/" || path == ""
	}

	// Exact match
	if path == pattern {
		return true
	}

	// Special handling for home category language prefixes
	// Language codes like /it, /en, /de should only match exact paths, not /it/something
	if category == CategoryHome && isLanguagePrefix(pattern) {
		// Only allow exact match for language prefixes
		return false
	}

	// Split path into segments, filtering out empty segments
	pathSegments := filterEmptySegments(strings.Split(strings.Trim(path, "/"), "/"))
	patternSegments := filterEmptySegments(strings.Split(strings.Trim(pattern, "/"), "/"))

	// Check if pattern segments appear consecutively in path
	// This handles cases like:
	// - "/en/contact" matches "/contact" (pattern: ["contact"], path: ["en", "contact"])
	// - "/contact/form" matches "/contact" (pattern: ["contact"], path: ["contact", "form"])
	// But NOT:
	// - "/contacto" matches "/contact" (pattern: ["contact"], path: ["contacto"])

	if len(patternSegments) == 0 {
		return false
	}

	// For single-segment patterns, check if that segment exists in path
	if len(patternSegments) == 1 {
		patternSeg := patternSegments[0]
		for _, pathSeg := range pathSegments {
			if pathSeg == patternSeg {
				return true
			}
		}
		return false
	}

	// For multi-segment patterns, check for consecutive match
	for i := 0; i <= len(pathSegments)-len(patternSegments); i++ {
		match := true
		for j := 0; j < len(patternSegments); j++ {
			if pathSegments[i+j] != patternSegments[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}

	return false
}

// languagePrefixRegex matches language code patterns like /it, /en, /en-us, /pt-br, etc.
// Pattern: /XX or /XX-YY where X and Y are lowercase letters
// This is only used for CategoryHome to ensure language prefixes match exactly, not as segments
var languagePrefixRegex = regexp.MustCompile(`^/[a-z]{2}(?:-[a-z]{2})?$`)

// isLanguagePrefix checks if a pattern is a language code or regional variant
// Returns true for patterns like /it, /en, /en-us, /pt-br, etc.
// Only used for CategoryHome to enforce exact matching of language prefixes
func isLanguagePrefix(pattern string) bool {
	return languagePrefixRegex.MatchString(pattern)
}

// filterEmptySegments removes empty strings from a slice of segments
// This handles malformed URLs with double slashes like "//contact"
func filterEmptySegments(segments []string) []string {
	result := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg != "" {
			result = append(result, seg)
		}
	}
	return result
}

// GetStandardPagesSummary returns a human-readable summary of categorized pages.
// This is useful for LLMs to quickly understand what standard pages were found.
//
// Example output:
//
//	"Found standard pages: home (1 URL), contact (2 URLs), about (1 URL), products (3 URLs)"
func GetStandardPagesSummary(categories map[PageCategory][]string) string {
	if len(categories) == 0 {
		return "No standard pages found"
	}

	var parts []string
	// Iterate in a consistent order
	categoryOrder := []PageCategory{
		CategoryHome, CategoryAbout, CategoryContact, CategoryProducts,
		CategoryBlog, CategoryFAQ, CategoryPrivacy, CategoryLogin, CategoryCart,
	}

	for _, cat := range categoryOrder {
		if urls, exists := categories[cat]; exists && len(urls) > 0 {
			count := len(urls)
			name := string(cat)
			if count == 1 {
				parts = append(parts, name+" (1 URL)")
			} else {
				parts = append(parts, name+" ("+strconv.Itoa(count)+" URLs)")
			}
		}
	}

	if len(parts) == 0 {
		return "No standard pages found"
	}

	return "Found standard pages: " + strings.Join(parts, ", ")
}
