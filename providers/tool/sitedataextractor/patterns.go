package sitedataextractor

import (
	"regexp"
	"strings"
)

// Compiled regex patterns for data extraction.
// Patterns are case-insensitive where appropriate.

// === EMAIL PATTERNS ===

// emailPattern matches standard email addresses.
var emailPattern = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

// pecPattern matches Italian PEC (certified email) addresses.
// PEC domains include: pec.it, legalmail.it, postecert.it, and other certified providers.
// Matches domains like: name@pec.it, name@company.pec.it, name@legalmail.it
var pecPattern = regexp.MustCompile(`(?i)[a-zA-Z0-9._%+\-]+@(?:[a-zA-Z0-9.\-]+\.)?(?:pec|legalmail|postecert|arubapec|pecveneto)\.it`)

// pecCompanySubdomainPattern matches company-specific PEC addresses with "pec" subdomain.
// Matches patterns like: name@pec.aecilluminazione.it, legal@pec.company.com
// This is common in Italy where companies create pec.companyname.tld subdomains.
var pecCompanySubdomainPattern = regexp.MustCompile(`(?i)[a-zA-Z0-9._%+\-]+@pec\.[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

// === PHONE PATTERNS ===

// phonePattern matches international phone numbers with various formats.
// Handles: +39 02 1234567, (02) 12345678, 0039-02-1234567, etc.
var phonePattern = regexp.MustCompile(`(?:(?:\+|00)\d{1,3}[\s\-.]?)?\(?\d{2,4}\)?[\s\-.]?\d{3,4}[\s\-.]?\d{3,4}`)

// faxLabelPattern matches "fax" keyword followed by a number.
var faxLabelPattern = regexp.MustCompile(`(?i)(?:fax|telefax|facsimile)[\s:.\-]*(\+?[\d\s\-().]{7,20})`)

// === VAT/TAX PATTERNS ===

// Italian VAT (Partita IVA): 11 digits, optionally prefixed with "IT"
var italianVATPattern = regexp.MustCompile(`(?i)(?:P\.?\s*IVA|partita\s*iva|vat)[\s:.\-]*(?:IT)?(\d{11})`)

// Generic European VAT: 2-letter country code + numbers
var euVATPattern = regexp.MustCompile(`(?i)(?:VAT|TVA|USt)[\s:.\-]*([A-Z]{2}\d{8,12})`)

// Italian Codice Fiscale: 16 alphanumeric characters (strict format)
// Format: 6 letters + 2 digits + 1 letter + 2 digits + 1 letter + 3 digits + 1 letter
var italianTaxCodePattern = regexp.MustCompile(`(?i)(?:C\.?\s*F\.?|codice\s*fiscale|fiscal\s*code)[\s:.\-]*([A-Z]{6}\d{2}[A-Z]\d{2}[A-Z]\d{3}[A-Z])`)

// Standalone Italian Tax Code (without label, stricter context)
var italianTaxCodeStandalonePattern = regexp.MustCompile(`\b([A-Z]{6}\d{2}[A-Z]\d{2}[A-Z]\d{3}[A-Z])\b`)

// === ADDRESS PATTERNS ===

// Street address patterns for multiple languages
var streetPatterns = []*regexp.Regexp{
	// Italian: Via Roma 123, Corso Italia 45/A, Piazza Duomo 1
	regexp.MustCompile(`(?i)((?:via|viale|corso|piazza|piazzale|largo|vicolo|contrada|strada|loc\.|localit[àa])\s+[A-Za-zÀ-ÿ\s'.]+[\s,]+\d+[/\-]?[A-Za-z]?)`),
	// English: 123 Main Street, 45 Oak Avenue
	regexp.MustCompile(`(?i)(\d+\s+[A-Za-z\s]+(?:street|st\.|avenue|ave\.|road|rd\.|boulevard|blvd\.|lane|ln\.|drive|dr\.|way|place|pl\.|court|ct\.))`),
	// German: Hauptstraße 123, Berliner Str. 45
	regexp.MustCompile(`(?i)([A-Za-zäöüÄÖÜß\s]+(?:straße|strasse|str\.|weg|platz|allee)\s*\d+[a-z]?)`),
	// French: 123 Rue de Paris, 45 Avenue des Champs
	regexp.MustCompile(`(?i)(\d+[,\s]+(?:rue|avenue|boulevard|place|chemin|allée)\s+[A-Za-zÀ-ÿ\s']+)`),
	// Spanish: Calle Mayor 123, Avenida de la Constitución 45
	regexp.MustCompile(`(?i)((?:calle|avenida|paseo|plaza|camino)\s+[A-Za-zÀ-ÿ\s]+[\s,]+\d+)`),
}

// ZIP/Postal code patterns by country
var zipCodePatterns = map[string]*regexp.Regexp{
	"IT": regexp.MustCompile(`\b(\d{5})\b`),                                // Italy: 5 digits
	"US": regexp.MustCompile(`\b(\d{5}(?:-\d{4})?)\b`),                     // US: 5 digits or 5+4
	"UK": regexp.MustCompile(`\b([A-Z]{1,2}\d{1,2}[A-Z]?\s*\d[A-Z]{2})\b`), // UK: complex format
	"DE": regexp.MustCompile(`\b(\d{5})\b`),                                // Germany: 5 digits
	"FR": regexp.MustCompile(`\b(\d{5})\b`),                                // France: 5 digits
	"ES": regexp.MustCompile(`\b(\d{5})\b`),                                // Spain: 5 digits
	"NL": regexp.MustCompile(`\b(\d{4}\s?[A-Z]{2})\b`),                     // Netherlands: 4 digits + 2 letters
	"CH": regexp.MustCompile(`\b(\d{4})\b`),                                // Switzerland: 4 digits
	"AT": regexp.MustCompile(`\b(\d{4})\b`),                                // Austria: 4 digits
	"BE": regexp.MustCompile(`\b(\d{4})\b`),                                // Belgium: 4 digits
	"PT": regexp.MustCompile(`\b(\d{4}(?:-\d{3})?)\b`),                     // Portugal: 4 digits or 4-3
	"CA": regexp.MustCompile(`\b([A-Z]\d[A-Z]\s?\d[A-Z]\d)\b`),             // Canada: A1A 1A1
	"AU": regexp.MustCompile(`\b(\d{4})\b`),                                // Australia: 4 digits
	"JP": regexp.MustCompile(`\b(\d{3}-\d{4})\b`),                          // Japan: 3-4 format
	"BR": regexp.MustCompile(`\b(\d{5}-\d{3})\b`),                          // Brazil: 5-3 format
}

// Italian regions by province
var provinceToRegion = map[string]string{
	"AG": "Sicilia", "AL": "Piemonte", "AN": "Marche", "AO": "Valle d'Aosta", "AR": "Toscana",
	"AP": "Marche", "AT": "Piemonte", "AV": "Campania", "BA": "Puglia", "BT": "Puglia",
	"BL": "Veneto", "BN": "Campania", "BG": "Lombardia", "BI": "Piemonte", "BO": "Emilia-Romagna",
	"BZ": "Trentino-Alto Adige", "BS": "Lombardia", "BR": "Puglia", "CA": "Sardegna", "CL": "Sicilia",
	"CB": "Molise", "CE": "Campania", "CT": "Sicilia", "CZ": "Calabria", "CH": "Abruzzo",
	"CO": "Lombardia", "CS": "Calabria", "CR": "Lombardia", "KR": "Calabria", "CN": "Piemonte",
	"EN": "Sicilia", "FM": "Marche", "FE": "Emilia-Romagna", "FI": "Toscana", "FG": "Puglia",
	"FC": "Emilia-Romagna", "FR": "Lazio", "GE": "Liguria", "GO": "Friuli-Venezia Giulia", "GR": "Toscana",
	"IM": "Liguria", "IS": "Molise", "SP": "Liguria", "AQ": "Abruzzo", "LT": "Lazio",
	"LE": "Puglia", "LC": "Lombardia", "LI": "Toscana", "LO": "Lombardia", "LU": "Toscana",
	"MC": "Marche", "MN": "Lombardia", "MS": "Toscana", "MT": "Basilicata", "ME": "Sicilia",
	"MI": "Lombardia", "MO": "Emilia-Romagna", "MB": "Lombardia", "NA": "Campania", "NO": "Piemonte",
	"NU": "Sardegna", "OR": "Sardegna", "PD": "Veneto", "PA": "Sicilia", "PR": "Emilia-Romagna",
	"PV": "Lombardia", "PG": "Umbria", "PU": "Marche", "PE": "Abruzzo", "PC": "Emilia-Romagna",
	"PI": "Toscana", "PT": "Toscana", "PN": "Friuli-Venezia Giulia", "PZ": "Basilicata", "PO": "Toscana",
	"RG": "Sicilia", "RA": "Emilia-Romagna", "RC": "Calabria", "RE": "Emilia-Romagna", "RI": "Lazio",
	"RN": "Emilia-Romagna", "RM": "Lazio", "RO": "Veneto", "SA": "Campania", "SS": "Sardegna",
	"SV": "Liguria", "SI": "Toscana", "SR": "Sicilia", "SO": "Lombardia", "SU": "Sardegna",
	"TA": "Puglia", "TE": "Abruzzo", "TR": "Umbria", "TO": "Piemonte", "TP": "Sicilia",
	"TN": "Trentino-Alto Adige", "TV": "Veneto", "TS": "Friuli-Venezia Giulia", "UD": "Friuli-Venezia Giulia", "VA": "Lombardia",
	"VE": "Veneto", "VB": "Piemonte", "VC": "Piemonte", "VR": "Veneto", "VV": "Calabria",
	"VI": "Veneto", "VT": "Lazio",
}

// zipPrefixToProvince maps Italian ZIP code (CAP) first 2 digits to province code.
// Italian CAP codes are 5 digits where the first 2 identify the province.
// Reference: Italian postal system (Poste Italiane)
var zipPrefixToProvince = map[string]string{
	// Lazio
	"00": "RM", "01": "VT", "02": "RI", "03": "LT", "04": "FR",
	// Piemonte
	"10": "TO", "11": "AO", "12": "CN", "13": "VC", "14": "AT", "15": "AL", "28": "VB",
	// Lombardia
	"20": "MI", "21": "VA", "22": "CO", "23": "SO", "24": "BG", "25": "BS", "26": "CR", "27": "PV", "46": "MN",
	// Trentino-Alto Adige
	"38": "TN", "39": "BZ",
	// Veneto
	"30": "VE", "31": "TV", "32": "BL", "35": "PD", "36": "VI", "37": "VR", "45": "RO",
	// Friuli-Venezia Giulia
	"33": "UD", "34": "TS",
	// Liguria
	"16": "GE", "17": "SV", "18": "IM", "19": "SP",
	// Emilia-Romagna
	"29": "PC", "40": "BO", "41": "MO", "42": "RE", "43": "PR", "44": "FE", "47": "FC", "48": "RA",
	// Toscana
	"50": "FI", "51": "PT", "52": "AR", "53": "SI", "54": "MS", "55": "LU", "56": "PI", "57": "LI", "58": "GR", "59": "PO",
	// Umbria
	"05": "TR", "06": "PG",
	// Marche
	"60": "AN", "61": "PU", "62": "MC", "63": "AP",
	// Abruzzo
	"64": "TE", "65": "PE", "66": "CH", "67": "AQ",
	// Molise
	"86": "CB",
	// Campania
	"80": "NA", "81": "CE", "82": "BN", "83": "AV", "84": "SA",
	// Puglia
	"70": "BA", "71": "FG", "72": "TA", "73": "LE", "74": "TA", "76": "BT",
	// Basilicata
	"75": "MT", "85": "PZ",
	// Calabria
	"87": "CS", "88": "CZ", "89": "RC",
	// Sicilia
	"90": "PA", "91": "TP", "92": "AG", "93": "CL", "94": "EN", "95": "CT", "96": "SR", "97": "RG", "98": "ME",
	// Sardegna
	"07": "SS", "08": "NU", "09": "CA",
}

// Country name to ISO code mapping
var countryToCode = map[string]string{
	// English names
	"italy": "IT", "united states": "US", "usa": "US", "united kingdom": "GB", "uk": "GB",
	"germany": "DE", "france": "FR", "spain": "ES", "netherlands": "NL", "belgium": "BE",
	"switzerland": "CH", "austria": "AT", "portugal": "PT", "poland": "PL", "sweden": "SE",
	"norway": "NO", "denmark": "DK", "finland": "FI", "ireland": "IE", "greece": "GR",
	"czech republic": "CZ", "czechia": "CZ", "romania": "RO", "hungary": "HU", "croatia": "HR",
	"slovakia": "SK", "slovenia": "SI", "bulgaria": "BG", "lithuania": "LT", "latvia": "LV",
	"estonia": "EE", "luxembourg": "LU", "malta": "MT", "cyprus": "CY",
	// Italian names
	"italia": "IT", "stati uniti": "US", "regno unito": "GB", "germania": "DE", "francia": "FR",
	"spagna": "ES", "paesi bassi": "NL", "belgio": "BE", "svizzera": "CH", "portogallo": "PT",
	"svezia": "SE", "norvegia": "NO", "danimarca": "DK", "finlandia": "FI", "irlanda": "IE",
	"grecia": "GR", "ungheria": "HU", "croazia": "HR",
	// German names
	"italien": "IT", "vereinigte staaten": "US", "vereinigtes königreich": "GB", "deutschland": "DE",
	"frankreich": "FR", "spanien": "ES", "niederlande": "NL", "belgien": "BE", "schweiz": "CH",
	"österreich": "AT", "schweden": "SE", "norwegen": "NO", "dänemark": "DK",
	// French names
	"italie": "IT", "états-unis": "US", "royaume-uni": "GB", "allemagne": "DE",
	"espagne": "ES", "pays-bas": "NL", "belgique": "BE", "suisse": "CH", "autriche": "AT",
	"suède": "SE", "norvège": "NO", "danemark": "DK",
}

// Country code to full name mapping
var codeToCountry = map[string]string{
	"IT": "Italy", "US": "United States", "GB": "United Kingdom", "DE": "Germany",
	"FR": "France", "ES": "Spain", "NL": "Netherlands", "BE": "Belgium", "CH": "Switzerland",
	"AT": "Austria", "PT": "Portugal", "PL": "Poland", "SE": "Sweden", "NO": "Norway",
	"DK": "Denmark", "FI": "Finland", "IE": "Ireland", "GR": "Greece", "CZ": "Czech Republic",
	"RO": "Romania", "HU": "Hungary", "HR": "Croatia", "SK": "Slovakia", "SI": "Slovenia",
	"BG": "Bulgaria", "LT": "Lithuania", "LV": "Latvia", "EE": "Estonia", "LU": "Luxembourg",
	"MT": "Malta", "CY": "Cyprus", "CA": "Canada", "AU": "Australia", "JP": "Japan", "BR": "Brazil",
}

// === SOCIAL MEDIA PATTERNS ===

// Social media URL patterns (must capture the full URL)
// Note: Go's regexp doesn't support negative lookahead (?!), so we use simpler patterns
// and filter out unwanted matches in code
var socialPatterns = map[string]*regexp.Regexp{
	"facebook":  regexp.MustCompile(`https?://(?:www\.)?facebook\.com/([a-zA-Z0-9.\-_]+/?)`),
	"linkedin":  regexp.MustCompile(`https?://(?:www\.)?linkedin\.com/company/([a-zA-Z0-9\-_]+/?)`),
	"twitter":   regexp.MustCompile(`https?://(?:www\.)?(?:twitter|x)\.com/([a-zA-Z0-9_]+/?)`),
	"instagram": regexp.MustCompile(`https?://(?:www\.)?instagram\.com/([a-zA-Z0-9._]+/?)`),
	"youtube":   regexp.MustCompile(`https?://(?:www\.)?youtube\.com/(?:channel|c|user|@)([a-zA-Z0-9\-_]+/?)`),
}

// socialExcludePatterns contains patterns to exclude from social media URLs
var socialExcludePatterns = map[string][]string{
	"facebook":  {"sharer", "share", "plugins", "dialog"},
	"twitter":   {"intent", "share"},
	"instagram": {"p/", "explore", "accounts"},
}

// LinkedIn company ID extraction (numeric ID from URL)
var linkedInIDPattern = regexp.MustCompile(`linkedin\.com/company/(\d+)`)

// === BUSINESS DATA PATTERNS ===

// ATECO/NACE code pattern (Italian activity classification)
// Format: XX.XX.XX or XX.XX or just XX
var atecoPattern = regexp.MustCompile(`(?i)(?:ateco|nace|codice\s*attivit[àa])[\s:.\-]*(\d{2}(?:\.\d{2}(?:\.\d{2})?)?)`)

// Employee count patterns
var employeesPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(?:dipendenti|employees|collaboratori|addetti|persone|workers|staff\s*members)`),
	regexp.MustCompile(`(?i)(?:dipendenti|employees|team|staff)[\s:]*(\d+(?:[.,]\d+)?)`),
	regexp.MustCompile(`(?i)(?:organico|workforce|headcount)[\s:]*(\d+(?:[.,]\d+)?)`),
}

// Turnover/Revenue patterns (with currency symbols and units)
var turnoverPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:fatturato|revenue|turnover|sales)[\s:]*[€$£]?\s*(\d+(?:[.,]\d+)?)\s*(?:milioni|mln|M|million)?`),
	regexp.MustCompile(`(?i)[€$£]\s*(\d+(?:[.,]\d+)?)\s*(?:milioni|mln|M|million)?\s*(?:di\s*fatturato|in\s*revenue|turnover)`),
}

// === LOGO PATTERNS ===

// Logo URL patterns in various attributes
var logoURLPatterns = []*regexp.Regexp{
	// Common logo filename patterns
	regexp.MustCompile(`(?i)/[^"'\s]*logo[^"'\s]*\.(?:png|jpg|jpeg|svg|gif|webp)`),
	// Header/brand images
	regexp.MustCompile(`(?i)/[^"'\s]*(?:brand|header|masthead)[^"'\s]*\.(?:png|jpg|jpeg|svg|gif|webp)`),
}

// logoClassKeywords contains class/ID keywords that typically indicate logos.
// Used by regex patterns in extractors.go to identify logo images.
var logoClassKeywords = []string{
	"logo", "brand", "site-logo", "header-logo", "navbar-logo", "masthead-logo",
	"company-logo", "main-logo", "custom-logo", "logo-image", "logo-img",
}

// ContainsLogoClass checks if a class string contains any logo-related keyword.
func ContainsLogoClass(class string) bool {
	classLower := strings.ToLower(class)
	for _, kw := range logoClassKeywords {
		if strings.Contains(classLower, kw) {
			return true
		}
	}
	return false
}

// === HELPER FUNCTIONS ===

// normalizeWhitespace collapses multiple whitespace characters into single spaces.
func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// cleanPhoneNumber normalizes a phone number by removing formatting.
func cleanPhoneNumber(phone string) string {
	// Remove all non-digit characters except leading +
	cleaned := strings.TrimSpace(phone)
	if strings.HasPrefix(cleaned, "+") {
		return "+" + regexp.MustCompile(`[^\d]`).ReplaceAllString(cleaned[1:], "")
	}
	return regexp.MustCompile(`[^\d]`).ReplaceAllString(cleaned, "")
}

// isValidEmail performs basic email validation.
func isValidEmail(email string) bool {
	if len(email) < 5 || len(email) > 254 {
		return false
	}
	return emailPattern.MatchString(email)
}

// isPECEmail checks if an email is a certified PEC address.
// Checks both standard PEC providers and company-specific PEC subdomains.
func isPECEmail(email string) bool {
	return pecPattern.MatchString(email) || pecCompanySubdomainPattern.MatchString(email)
}

// getRegionFromProvince returns the region name for an Italian province code.
func getRegionFromProvince(code string) string {
	return provinceToRegion[strings.ToUpper(code)]
}

// getProvinceFromZIP returns the province code for an Italian ZIP code (CAP).
// Returns empty string if the ZIP is not recognized as Italian.
func getProvinceFromZIP(zip string) string {
	zip = strings.TrimSpace(zip)
	if len(zip) != 5 {
		return ""
	}
	// Check if all characters are digits
	for _, c := range zip {
		if c < '0' || c > '9' {
			return ""
		}
	}
	prefix := zip[:2]
	return zipPrefixToProvince[prefix]
}

// getCountryCode returns the ISO country code for a country name.
func getCountryCode(name string) string {
	return countryToCode[strings.ToLower(strings.TrimSpace(name))]
}

// getCountryName returns the full country name for an ISO country code.
func getCountryName(code string) string {
	return codeToCountry[strings.ToUpper(code)]
}

// isValidVAT performs basic VAT number validation.
func isValidVAT(vat string) bool {
	// Remove spaces and common prefixes
	cleaned := regexp.MustCompile(`[^A-Z0-9]`).ReplaceAllString(strings.ToUpper(vat), "")

	// Italian VAT: 11 digits
	if len(cleaned) == 11 && regexp.MustCompile(`^\d{11}$`).MatchString(cleaned) {
		return true
	}

	// European VAT: 2-letter prefix + 8-12 digits
	if len(cleaned) >= 10 && len(cleaned) <= 14 {
		if regexp.MustCompile(`^[A-Z]{2}\d{8,12}$`).MatchString(cleaned) {
			return true
		}
	}

	return false
}

// isValidTaxCode validates an Italian Codice Fiscale.
func isValidTaxCode(code string) bool {
	cleaned := strings.ToUpper(strings.TrimSpace(code))
	if len(cleaned) != 16 {
		return false
	}
	return italianTaxCodeStandalonePattern.MatchString(cleaned)
}
