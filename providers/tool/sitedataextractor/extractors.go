package sitedataextractor

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// parseHTMLPage parses an HTML document and extracts structured data.
// It extracts JSON-LD blocks, meta tags, link elements, and visible text content.
func parseHTMLPage(htmlContent string, baseURL *url.URL) *pageData {
	data := &pageData{
		html:     htmlContent,
		jsonLD:   make([]map[string]interface{}, 0),
		metaTags: make(map[string]string),
		links:    make([]linkTag, 0),
		scripts:  make([]string, 0),
	}

	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return data
	}

	var textBuilder strings.Builder
	var parseNode func(*html.Node)
	parseNode = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script":
				// Check for JSON-LD
				for _, attr := range n.Attr {
					if attr.Key == "type" && attr.Val == "application/ld+json" {
						if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
							var jsonData map[string]interface{}
							if err := json.Unmarshal([]byte(n.FirstChild.Data), &jsonData); err == nil {
								data.jsonLD = append(data.jsonLD, jsonData)
							}
							// Try as array
							var jsonArray []map[string]interface{}
							if err := json.Unmarshal([]byte(n.FirstChild.Data), &jsonArray); err == nil {
								data.jsonLD = append(data.jsonLD, jsonArray...)
							}
						}
						return // Don't recurse into script content
					}
				}
				// Store other scripts for potential data extraction
				if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
					data.scripts = append(data.scripts, n.FirstChild.Data)
				}
				return // Don't recurse into script content

			case "meta":
				name := ""
				property := ""
				content := ""
				for _, attr := range n.Attr {
					switch attr.Key {
					case "name":
						name = attr.Val
					case "property":
						property = attr.Val
					case "content":
						content = attr.Val
					}
				}
				if content != "" {
					if property != "" {
						data.metaTags[property] = content
					}
					if name != "" {
						data.metaTags[name] = content
					}
				}

			case "link":
				link := linkTag{}
				for _, attr := range n.Attr {
					switch attr.Key {
					case "rel":
						link.rel = attr.Val
					case "href":
						link.href = attr.Val
					case "type":
						link.typ = attr.Val
					case "sizes":
						link.sizes = attr.Val
					}
				}
				if link.href != "" {
					// Resolve relative URLs
					link.href = resolveURL(link.href, baseURL)
					data.links = append(data.links, link)
				}

			case "style", "noscript":
				return // Skip these elements
			}
		}

		// Extract visible text
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				textBuilder.WriteString(text)
				textBuilder.WriteString(" ")
			}
		}

		// Recurse into children
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			parseNode(c)
		}
	}

	parseNode(doc)
	data.text = textBuilder.String()

	return data
}

// resolveURL converts a relative URL to absolute using the base URL.
// It handles protocol-relative URLs, relative paths, and absolute URLs.
func resolveURL(href string, baseURL *url.URL) string {
	if baseURL == nil {
		return href
	}

	if strings.HasPrefix(href, "//") {
		// Protocol-relative URL
		return baseURL.Scheme + ":" + href
	}

	if strings.HasPrefix(href, "/") || !strings.Contains(href, "://") {
		// Relative URL
		parsed, err := url.Parse(href)
		if err != nil {
			return href
		}
		return baseURL.ResolveReference(parsed).String()
	}

	return href
}

// === LOGO EXTRACTION ===

// extractLogo extracts the company logo URL from the page.
// It uses multiple extraction sources in priority order:
// JSON-LD Organization.logo (0.95), img with logo class/id (0.85),
// apple-touch-icon (0.85), SVG/PNG favicons (0.78-0.75), og:image (0.50),
// URL patterns (0.50), and favicon.ico (0.30).
// Results are stored in output.LogoURL with confidence scores.
func extractLogo(ctx *extractionContext, page *pageData, output *Output) {
	// 1. Check JSON-LD for Organization logo
	for _, jsonData := range page.jsonLD {
		if logo := extractJSONLDLogo(jsonData); logo != "" {
			logo = resolveURL(logo, ctx.baseURL)
			updateFieldIfBetter(&output.LogoURL, ExtractedField{
				Value:      logo,
				Confidence: 0.95,
				Source:     "json-ld",
				PageSource: page.category,
			})
			ctx.sourcesUsed["json-ld"] = true
			return // Highest confidence, no need to continue
		}
	}

	// 2. Look for <img> with logo class/id using regex on HTML (high priority)
	imgLogoPattern := regexp.MustCompile(`(?i)<img[^>]*(?:class|id)=['""][^'""]*(?:logo|brand)[^'""]*['""][^>]*src=['""]([^'""]+)['""]`)
	imgLogoPattern2 := regexp.MustCompile(`(?i)<img[^>]*src=['""]([^'""]+)['""][^>]*(?:class|id)=['""][^'""]*(?:logo|brand)[^'""]*['""]`)

	if matches := imgLogoPattern.FindStringSubmatch(page.html); len(matches) > 1 {
		logoURL := resolveURL(matches[1], ctx.baseURL)
		updateFieldIfBetter(&output.LogoURL, ExtractedField{
			Value:      logoURL,
			Confidence: 0.85,
			Source:     "dom",
			PageSource: page.category,
		})
		ctx.sourcesUsed["dom"] = true
	} else if matches := imgLogoPattern2.FindStringSubmatch(page.html); len(matches) > 1 {
		logoURL := resolveURL(matches[1], ctx.baseURL)
		updateFieldIfBetter(&output.LogoURL, ExtractedField{
			Value:      logoURL,
			Confidence: 0.85,
			Source:     "dom",
			PageSource: page.category,
		})
		ctx.sourcesUsed["dom"] = true
	}

	// 3-6. Check link tags for icons
	for _, link := range page.links {
		rel := strings.ToLower(link.rel)
		hrefLower := strings.ToLower(link.href)

		switch {
		case strings.Contains(rel, "apple-touch-icon"):
			// Apple touch icons are typically the company logo
			confidence := 0.85
			// Boost if filename contains "logo"
			if containsLogoKeyword(hrefLower) {
				confidence = 0.88
			}
			updateFieldIfBetter(&output.LogoURL, ExtractedField{
				Value:      link.href,
				Confidence: confidence,
				Source:     "link-href",
				PageSource: page.category,
			})
			ctx.sourcesUsed["link-href"] = true

		case strings.Contains(rel, "icon") && strings.HasSuffix(hrefLower, ".svg"):
			// SVG favicons - boost if contains "logo" in path
			confidence := 0.78
			if containsLogoKeyword(hrefLower) {
				confidence = 0.82
			}
			updateFieldIfBetter(&output.LogoURL, ExtractedField{
				Value:      link.href,
				Confidence: confidence,
				Source:     "link-href",
				PageSource: page.category,
			})
			ctx.sourcesUsed["link-href"] = true

		case strings.Contains(rel, "icon") && (strings.HasSuffix(hrefLower, ".png") || link.typ == "image/png"):
			confidence := 0.75
			// Boost if filename contains "logo"
			if containsLogoKeyword(hrefLower) {
				confidence = 0.80
			}
			updateFieldIfBetter(&output.LogoURL, ExtractedField{
				Value:      link.href,
				Confidence: confidence,
				Source:     "link-href",
				PageSource: page.category,
			})
			ctx.sourcesUsed["link-href"] = true

		case rel == "icon" || rel == "shortcut icon":
			// Generic favicon
			confidence := 0.30
			// Boost if filename contains "logo"
			if containsLogoKeyword(hrefLower) {
				confidence = 0.55
			}
			updateFieldIfBetter(&output.LogoURL, ExtractedField{
				Value:      link.href,
				Confidence: confidence,
				Source:     "link-href",
				PageSource: page.category,
			})
			ctx.sourcesUsed["link-href"] = true
		}
	}

	// 7. Check og:image (only if from same domain and NOT a marketing image)
	// og:image often contains marketing photos, not logos, so use lower confidence
	if ogImage := page.metaTags["og:image"]; ogImage != "" {
		ogImage = resolveURL(ogImage, ctx.baseURL)
		if isSameDomain(ogImage, ctx.baseURL) && !isLikelyMarketingImage(ogImage) {
			confidence := 0.50
			// Boost if URL contains "logo"
			if containsLogoKeyword(strings.ToLower(ogImage)) {
				confidence = 0.70
			}
			updateFieldIfBetter(&output.LogoURL, ExtractedField{
				Value:      ogImage,
				Confidence: confidence,
				Source:     "og:tag",
				PageSource: page.category,
			})
			ctx.sourcesUsed["og:tag"] = true
		}
	}

	// 8. URL pattern matching for logo files
	for _, pattern := range logoURLPatterns {
		if matches := pattern.FindStringSubmatch(page.html); len(matches) > 0 {
			logoURL := resolveURL(matches[0], ctx.baseURL)
			updateFieldIfBetter(&output.LogoURL, ExtractedField{
				Value:      logoURL,
				Confidence: 0.50,
				Source:     "regex",
				PageSource: page.category,
			})
			ctx.sourcesUsed["regex"] = true
		}
	}
}

// containsLogoKeyword checks if a URL path contains logo-related keywords.
// It returns true if the path contains keywords like "logo", "brand", "emblem", or "marchio".
func containsLogoKeyword(path string) bool {
	keywords := []string{"logo", "brand", "emblem", "marchio"}
	for _, kw := range keywords {
		if strings.Contains(path, kw) {
			return true
		}
	}
	return false
}

// isLikelyMarketingImage checks if an image URL appears to be a marketing photo rather than a logo.
// It returns true if the image URL contains keywords or size indicators that suggest
// a large marketing photo (e.g., "product", "team", "1920x1080") rather than a logo.
func isLikelyMarketingImage(imageURL string) bool {
	path := strings.ToLower(imageURL)

	// First check if it's clearly a logo - if so, don't filter it
	logoKeywords := []string{"logo", "brand", "emblem", "marchio", "favicon", "icon"}
	for _, kw := range logoKeywords {
		if strings.Contains(path, kw) {
			return false // Not a marketing image if it has logo keywords
		}
	}

	// Keywords that indicate marketing/product photos rather than logos
	excludeKeywords := []string{
		"laboratory", "laboratori", "product", "prodott", "team", "staff",
		"photo", "foto", "banner", "hero", "slide",
		"gallery", "galleria", "background", "sfondo", "cover", "copertina",
		"screenshot", "preview", "thumbnail", "thumb", "warehouse", "magazzino",
		"factory", "fabbrica", "office", "ufficio", "building", "edificio",
	}

	for _, kw := range excludeKeywords {
		if strings.Contains(path, kw) {
			return true
		}
	}

	// Size indicators in filename that suggest a large photo (not typical for logos)
	sizePatterns := []string{
		"-scaled", "-resize", "-large", "-full", "-big",
		"1024x", "1280x", "1920x", "2048x", "800x", "600x",
		"x1024", "x1280", "x1920", "x2048", "x800", "x600",
	}

	for _, pattern := range sizePatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	return false
}

// extractJSONLDLogo extracts a logo URL from JSON-LD structured data.
// It looks for Organization or similar entity types and extracts the logo or image field.
// Returns the logo URL string or an empty string if not found.
func extractJSONLDLogo(data map[string]interface{}) string {
	// Check @type for Organization, LocalBusiness, etc.
	typeVal, ok := data["@type"].(string)
	if !ok || typeVal == "" {
		// Check for array of types
		if types, typesOK := data["@type"].([]interface{}); typesOK && len(types) > 0 {
			typeVal, _ = types[0].(string) //nolint:errcheck // Type assertion can fail safely
		}
	}

	orgTypes := []string{"Organization", "LocalBusiness", "Corporation", "Company", "NGO", "Brand"}
	isOrgType := false
	for _, t := range orgTypes {
		if strings.EqualFold(typeVal, t) {
			isOrgType = true
			break
		}
	}

	if !isOrgType {
		return ""
	}

	// Try different logo field formats
	if logo, ok := data["logo"].(string); ok {
		return logo
	}
	if logoObj, ok := data["logo"].(map[string]interface{}); ok {
		if url, ok := logoObj["url"].(string); ok {
			return url
		}
		if url, ok := logoObj["@id"].(string); ok {
			return url
		}
	}
	if image, ok := data["image"].(string); ok {
		return image
	}
	if imageObj, ok := data["image"].(map[string]interface{}); ok {
		if url, ok := imageObj["url"].(string); ok {
			return url
		}
	}

	return ""
}

// === CONTACT EXTRACTION ===

// extractContacts extracts email, phone, fax, and PEC contact information from the page.
// It uses JSON-LD data, regex patterns on visible text, mailto: links, and tel: links.
// Results are stored in output with confidence scores.
func extractContacts(ctx *extractionContext, page *pageData, output *Output) {
	// Extract from JSON-LD first
	for _, jsonData := range page.jsonLD {
		extractJSONLDContacts(jsonData, page.category, output)
		ctx.sourcesUsed["json-ld"] = true
	}

	// Extract emails from page text
	emails := emailPattern.FindAllString(page.text, -1)
	for _, email := range emails {
		if !isValidEmail(email) {
			continue
		}

		email = strings.ToLower(email)

		// Check if it's a PEC
		if isPECEmail(email) {
			updateFieldIfBetter(&output.PEC, ExtractedField{
				Value:      email,
				Confidence: 0.85,
				Source:     "regex",
				PageSource: page.category,
			})
			ctx.sourcesUsed["regex"] = true
		} else if isSameDomain(email, ctx.baseURL) {
			// Prefer emails from same domain
			// Apply confidence boost based on email prefix
			confidence := getEmailConfidenceByPrefix(email, 0.90)
			updateFieldIfBetter(&output.Email, ExtractedField{
				Value:      email,
				Confidence: confidence,
				Source:     "regex",
				PageSource: page.category,
			})
			ctx.sourcesUsed["regex"] = true
		} else {
			confidence := getEmailConfidenceByPrefix(email, 0.70)
			updateFieldIfBetter(&output.Email, ExtractedField{
				Value:      email,
				Confidence: confidence,
				Source:     "regex",
				PageSource: page.category,
			})
			ctx.sourcesUsed["regex"] = true
		}
	}

	// Extract from mailto: links
	mailtoPattern := regexp.MustCompile(`mailto:([a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,})`)
	if matches := mailtoPattern.FindAllStringSubmatch(page.html, -1); len(matches) > 0 {
		for _, match := range matches {
			email := strings.ToLower(match[1])
			if isPECEmail(email) {
				updateFieldIfBetter(&output.PEC, ExtractedField{
					Value:      email,
					Confidence: 0.90,
					Source:     "link-href",
					PageSource: page.category,
				})
			} else {
				updateFieldIfBetter(&output.Email, ExtractedField{
					Value:      email,
					Confidence: 0.95,
					Source:     "link-href",
					PageSource: page.category,
				})
			}
			ctx.sourcesUsed["link-href"] = true
		}
	}

	// Extract phone numbers
	phones := phonePattern.FindAllString(page.text, -1)
	for _, phone := range phones {
		cleaned := cleanPhoneNumber(phone)
		if len(cleaned) < 7 || len(cleaned) > 15 {
			continue // Invalid phone length
		}

		// Skip numbers that look like Italian VAT (11 digits without international prefix)
		// VAT numbers are 11 digits and don't have + or 00 prefix
		if isLikelyVATNotPhone(phone, cleaned) {
			continue
		}

		// Determine confidence based on format
		confidence := 0.75
		// Boost confidence for numbers with international prefix (+39, 0039, etc.)
		if strings.HasPrefix(phone, "+") || strings.HasPrefix(phone, "00") {
			confidence = 0.85
		}

		updateFieldIfBetter(&output.Phone, ExtractedField{
			Value:      phone,
			Confidence: confidence,
			Source:     "regex",
			PageSource: page.category,
		})
		ctx.sourcesUsed["regex"] = true
	}

	// Extract from tel: links
	telPattern := regexp.MustCompile(`tel:([+\d\-\s().]+)`)
	if matches := telPattern.FindAllStringSubmatch(page.html, -1); len(matches) > 0 {
		for _, match := range matches {
			phone := strings.TrimSpace(match[1])
			updateFieldIfBetter(&output.Phone, ExtractedField{
				Value:      phone,
				Confidence: 0.90,
				Source:     "link-href",
				PageSource: page.category,
			})
			ctx.sourcesUsed["link-href"] = true
		}
	}

	// Extract fax numbers (with fax label)
	if matches := faxLabelPattern.FindAllStringSubmatch(page.text, -1); len(matches) > 0 {
		for _, match := range matches {
			fax := strings.TrimSpace(match[1])
			updateFieldIfBetter(&output.Fax, ExtractedField{
				Value:      fax,
				Confidence: 0.80,
				Source:     "regex",
				PageSource: page.category,
			})
			ctx.sourcesUsed["regex"] = true
		}
	}
}

// extractJSONLDContacts extracts contact information from JSON-LD structured data.
// It extracts telephone, email, and fax from contactPoint or top-level fields.
// Results are stored in output fields with high confidence (0.95).
func extractJSONLDContacts(data map[string]interface{}, pageCategory string, output *Output) {
	// Check contactPoint
	if contactPoint, ok := data["contactPoint"].(map[string]interface{}); ok {
		if phone, ok := contactPoint["telephone"].(string); ok {
			updateFieldIfBetter(&output.Phone, ExtractedField{
				Value:      phone,
				Confidence: 0.95,
				Source:     "json-ld",
				PageSource: pageCategory,
			})
		}
		if email, ok := contactPoint["email"].(string); ok {
			updateFieldIfBetter(&output.Email, ExtractedField{
				Value:      email,
				Confidence: 0.95,
				Source:     "json-ld",
				PageSource: pageCategory,
			})
		}
	}

	// Check top-level telephone and email
	if phone, ok := data["telephone"].(string); ok {
		updateFieldIfBetter(&output.Phone, ExtractedField{
			Value:      phone,
			Confidence: 0.95,
			Source:     "json-ld",
			PageSource: pageCategory,
		})
	}
	if email, ok := data["email"].(string); ok {
		updateFieldIfBetter(&output.Email, ExtractedField{
			Value:      email,
			Confidence: 0.95,
			Source:     "json-ld",
			PageSource: pageCategory,
		})
	}
	if fax, ok := data["faxNumber"].(string); ok {
		updateFieldIfBetter(&output.Fax, ExtractedField{
			Value:      fax,
			Confidence: 0.95,
			Source:     "json-ld",
			PageSource: pageCategory,
		})
	}
}

// === SOCIAL LINKS EXTRACTION ===

// extractSocialLinks extracts social media profile URLs from JSON-LD sameAs and page links.
// It categorizes URLs into Facebook, LinkedIn, Twitter, Instagram, and YouTube fields.
// Extracts LinkedIn numeric company IDs when available.
func extractSocialLinks(ctx *extractionContext, page *pageData, output *Output) {
	// Extract from JSON-LD sameAs property
	for _, jsonData := range page.jsonLD {
		if sameAs, ok := jsonData["sameAs"].([]interface{}); ok {
			for _, urlFromJSON := range sameAs {
				if urlStr, ok := urlFromJSON.(string); ok {
					categorizeSocialURL(urlStr, page.category, "json-ld", 0.95, output)
					ctx.sourcesUsed["json-ld"] = true
				}
			}
		}
	}

	// Extract social links from HTML
	hrefPattern := regexp.MustCompile(`href=['""]([^'""]+)['""]`)
	hrefs := hrefPattern.FindAllStringSubmatch(page.html, -1)

	for _, match := range hrefs {
		href := match[1]
		categorizeSocialURL(href, page.category, "link-href", 0.85, output)
		ctx.sourcesUsed["link-href"] = true
	}

	// Extract LinkedIn numeric ID if we have a LinkedIn URL
	if output.LinkedIn.Value != "" {
		if matches := linkedInIDPattern.FindStringSubmatch(output.LinkedIn.Value); len(matches) > 1 {
			updateFieldIfBetter(&output.LinkedInID, ExtractedField{
				Value:      matches[1],
				Confidence: 0.95,
				Source:     output.LinkedIn.Source,
				PageSource: output.LinkedIn.PageSource,
			})
		}
	}
}

// categorizeSocialURL categorizes a URL into the appropriate social media field.
// It matches against known social network patterns and updates the corresponding output field.
// Applies exclude patterns to filter out sharing dialogs and other non-profile URLs.
func categorizeSocialURL(urlStr, pageCategory, source string, confidence float64, output *Output) {
	urlStr = strings.TrimSpace(urlStr)

	for network, pattern := range socialPatterns {
		if !pattern.MatchString(urlStr) {
			continue
		}

		// Check exclude patterns
		if excludes, ok := socialExcludePatterns[network]; ok {
			excluded := false
			for _, exclude := range excludes {
				if strings.Contains(urlStr, exclude) {
					excluded = true
					break
				}
			}
			if excluded {
				continue
			}
		}

		field := ExtractedField{
			Value:      urlStr,
			Confidence: confidence,
			Source:     source,
			PageSource: pageCategory,
		}

		switch network {
		case "facebook":
			updateFieldIfBetter(&output.Facebook, field)
		case "linkedin":
			updateFieldIfBetter(&output.LinkedIn, field)
		case "twitter":
			updateFieldIfBetter(&output.Twitter, field)
		case "instagram":
			updateFieldIfBetter(&output.Instagram, field)
		case "youtube":
			updateFieldIfBetter(&output.YouTube, field)
		}
		return
	}
}

// === ADDRESS EXTRACTION ===

// extractAddress extracts location data including address, city, ZIP code, and country.
// It uses JSON-LD structured data and regex patterns for address, ZIP codes, and derived region information.
// Results are stored in output with confidence scores based on the extraction method.
func extractAddress(ctx *extractionContext, page *pageData, output *Output) {
	// Extract from JSON-LD first (highest confidence)
	for _, jsonData := range page.jsonLD {
		extractJSONLDAddress(jsonData, page.category, output)
		ctx.sourcesUsed["json-ld"] = true
	}

	// Extract using regex patterns
	for _, pattern := range streetPatterns {
		if matches := pattern.FindStringSubmatch(page.text); len(matches) > 0 {
			address := normalizeWhitespace(matches[1])
			updateFieldIfBetter(&output.Address, ExtractedField{
				Value:      address,
				Confidence: 0.65,
				Source:     "regex",
				PageSource: page.category,
			})
			ctx.sourcesUsed["regex"] = true
			break
		}
	}

	// Extract ZIP codes
	for country, pattern := range zipCodePatterns {
		if matches := pattern.FindStringSubmatch(page.text); len(matches) > 1 {
			zip := matches[1]
			// Validate Italian ZIP codes are in proper range (00100-99100)
			if country == "IT" {
				if len(zip) == 5 && zip[0] >= '0' && zip[0] <= '9' {
					updateFieldIfBetter(&output.ZipCode, ExtractedField{
						Value:      zip,
						Confidence: 0.70,
						Source:     "regex",
						PageSource: page.category,
					})
					// Also set country code if we found an Italian ZIP
					updateFieldIfBetter(&output.CountryCode, ExtractedField{
						Value:      "IT",
						Confidence: 0.60,
						Source:     "regex",
						PageSource: page.category,
					})
					updateFieldIfBetter(&output.State, ExtractedField{
						Value:      "Italy",
						Confidence: 0.60,
						Source:     "regex",
						PageSource: page.category,
					})

					// Try to derive region from ZIP code using prefix mapping
					if provinceCode := getProvinceFromZIP(zip); provinceCode != "" {
						if regionName := getRegionFromProvince(provinceCode); regionName != "" {
							updateFieldIfBetter(&output.Region, ExtractedField{
								Value:      regionName,
								Confidence: 0.55, // Lower confidence since derived from ZIP
								Source:     "zip-lookup",
								PageSource: page.category,
							})
							ctx.sourcesUsed["zip-lookup"] = true
						}
					}

					ctx.sourcesUsed["regex"] = true
				}
			}
		}
	}

}

// extractJSONLDAddress extracts address information from JSON-LD structured data.
// It extracts street address, postal code, city, region, and country from address or location objects.
// Derives region names from Italian province codes and country names from ISO codes.
func extractJSONLDAddress(data map[string]interface{}, pageCategory string, output *Output) {
	var addr map[string]interface{}

	// Check for address field
	if a, ok := data["address"].(map[string]interface{}); ok {
		addr = a
	} else if loc, ok := data["location"].(map[string]interface{}); ok {
		if a, ok := loc["address"].(map[string]interface{}); ok {
			addr = a
		}
	}

	if addr == nil {
		return
	}

	confidence := 0.95

	if street, ok := addr["streetAddress"].(string); ok && street != "" {
		updateFieldIfBetter(&output.Address, ExtractedField{
			Value:      street,
			Confidence: confidence,
			Source:     "json-ld",
			PageSource: pageCategory,
		})
	}

	if zip, ok := addr["postalCode"].(string); ok && zip != "" {
		updateFieldIfBetter(&output.ZipCode, ExtractedField{
			Value:      zip,
			Confidence: confidence,
			Source:     "json-ld",
			PageSource: pageCategory,
		})
	}

	if city, ok := addr["addressLocality"].(string); ok && city != "" {
		updateFieldIfBetter(&output.City, ExtractedField{
			Value:      city,
			Confidence: confidence,
			Source:     "json-ld",
			PageSource: pageCategory,
		})
	}

	if region, ok := addr["addressRegion"].(string); ok && region != "" {
		// Check if it's a province code or full name
		if len(region) == 2 {
			// It's a province code, try to get region name
			if regionName := getRegionFromProvince(region); regionName != "" {
				updateFieldIfBetter(&output.Region, ExtractedField{
					Value:      regionName,
					Confidence: confidence,
					Source:     "json-ld",
					PageSource: pageCategory,
				})
			}
		} else {
			updateFieldIfBetter(&output.Region, ExtractedField{
				Value:      region,
				Confidence: confidence,
				Source:     "json-ld",
				PageSource: pageCategory,
			})
		}
	}

	if country, ok := addr["addressCountry"].(string); ok && country != "" {
		if len(country) == 2 {
			updateFieldIfBetter(&output.CountryCode, ExtractedField{
				Value:      strings.ToUpper(country),
				Confidence: confidence,
				Source:     "json-ld",
				PageSource: pageCategory,
			})
			if countryName := getCountryName(country); countryName != "" {
				updateFieldIfBetter(&output.State, ExtractedField{
					Value:      countryName,
					Confidence: confidence,
					Source:     "json-ld",
					PageSource: pageCategory,
				})
			}
		} else {
			updateFieldIfBetter(&output.State, ExtractedField{
				Value:      country,
				Confidence: confidence,
				Source:     "json-ld",
				PageSource: pageCategory,
			})
			if code := getCountryCode(country); code != "" {
				updateFieldIfBetter(&output.CountryCode, ExtractedField{
					Value:      code,
					Confidence: confidence,
					Source:     "json-ld",
					PageSource: pageCategory,
				})
			}
		}
	}
}

// === COMPANY DATA EXTRACTION ===

// extractCompanyData extracts company name, VAT, and tax code from the page.
// It uses JSON-LD data, Open Graph meta tags, and regex patterns for Italian and European VAT and tax codes.
// Results are stored in output with appropriate confidence scores.
func extractCompanyData(ctx *extractionContext, page *pageData, output *Output) {
	// Extract from JSON-LD
	for _, jsonData := range page.jsonLD {
		extractJSONLDCompanyData(jsonData, page.category, output)
		ctx.sourcesUsed["json-ld"] = true
	}

	// Extract from meta tags
	if siteName := page.metaTags["og:site_name"]; siteName != "" {
		updateFieldIfBetter(&output.CompanyName, ExtractedField{
			Value:      siteName,
			Confidence: 0.85,
			Source:     "og:tag",
			PageSource: page.category,
		})
		ctx.sourcesUsed["og:tag"] = true
	}

	// Extract VAT numbers
	if matches := italianVATPattern.FindStringSubmatch(page.text); len(matches) > 1 {
		vat := matches[1]
		if isValidVAT(vat) {
			updateFieldIfBetter(&output.VAT, ExtractedField{
				Value:      vat,
				Confidence: 0.90,
				Source:     "regex",
				PageSource: page.category,
			})
			ctx.sourcesUsed["regex"] = true
		}
	}

	if matches := euVATPattern.FindStringSubmatch(page.text); len(matches) > 1 {
		vat := matches[1]
		if isValidVAT(vat) {
			updateFieldIfBetter(&output.VAT, ExtractedField{
				Value:      vat,
				Confidence: 0.85,
				Source:     "regex",
				PageSource: page.category,
			})
			ctx.sourcesUsed["regex"] = true
		}
	}

	// Extract Italian tax code
	if matches := italianTaxCodePattern.FindStringSubmatch(page.text); len(matches) > 1 {
		taxCode := strings.ToUpper(matches[1])
		if isValidTaxCode(taxCode) {
			updateFieldIfBetter(&output.TaxCode, ExtractedField{
				Value:      taxCode,
				Confidence: 0.90,
				Source:     "regex",
				PageSource: page.category,
			})
			ctx.sourcesUsed["regex"] = true
		}
	}

	// Also try standalone pattern for tax code
	if matches := italianTaxCodeStandalonePattern.FindStringSubmatch(page.text); len(matches) > 1 {
		taxCode := strings.ToUpper(matches[1])
		if isValidTaxCode(taxCode) {
			updateFieldIfBetter(&output.TaxCode, ExtractedField{
				Value:      taxCode,
				Confidence: 0.70,
				Source:     "regex",
				PageSource: page.category,
			})
			ctx.sourcesUsed["regex"] = true
		}
	}
}

// extractJSONLDCompanyData extracts company data from JSON-LD structured data.
// It extracts name, legal name, description, VAT, tax ID, industry, and employee count.
// Results are stored in output with high confidence scores (0.95+).
func extractJSONLDCompanyData(data map[string]interface{}, pageCategory string, output *Output) {
	// Check @type for Organization types (unused but kept for potential future filtering)
	_, _ = data["@type"].(string) //nolint:errcheck // Type assertion result not needed here

	// Extract name
	if name, ok := data["name"].(string); ok && name != "" {
		updateFieldIfBetter(&output.CompanyName, ExtractedField{
			Value:      name,
			Confidence: 0.95,
			Source:     "json-ld",
			PageSource: pageCategory,
		})
	}
	if legalName, ok := data["legalName"].(string); ok && legalName != "" {
		updateFieldIfBetter(&output.CompanyName, ExtractedField{
			Value:      legalName,
			Confidence: 0.98, // Legal name is more authoritative
			Source:     "json-ld",
			PageSource: pageCategory,
		})
	}

	// Extract description
	if desc, ok := data["description"].(string); ok && desc != "" {
		updateFieldIfBetter(&output.Description, ExtractedField{
			Value:      desc,
			Confidence: 0.95,
			Source:     "json-ld",
			PageSource: pageCategory,
		})
	}

	// Extract VAT
	if vatID, ok := data["vatID"].(string); ok && vatID != "" {
		updateFieldIfBetter(&output.VAT, ExtractedField{
			Value:      vatID,
			Confidence: 0.95,
			Source:     "json-ld",
			PageSource: pageCategory,
		})
	}
	if taxID, ok := data["taxID"].(string); ok && taxID != "" {
		// Could be VAT or tax code depending on format
		if len(taxID) == 16 && isValidTaxCode(taxID) {
			updateFieldIfBetter(&output.TaxCode, ExtractedField{
				Value:      taxID,
				Confidence: 0.95,
				Source:     "json-ld",
				PageSource: pageCategory,
			})
		} else if isValidVAT(taxID) {
			updateFieldIfBetter(&output.VAT, ExtractedField{
				Value:      taxID,
				Confidence: 0.95,
				Source:     "json-ld",
				PageSource: pageCategory,
			})
		}
	}

	// Extract industry
	if industry, ok := data["industry"].(string); ok && industry != "" {
		updateFieldIfBetter(&output.Industry, ExtractedField{
			Value:      industry,
			Confidence: 0.90,
			Source:     "json-ld",
			PageSource: pageCategory,
		})
	}

	// Extract number of employees
	if employees, ok := data["numberOfEmployees"].(map[string]interface{}); ok {
		if value, ok := employees["value"].(float64); ok {
			updateFieldIfBetter(&output.Employees, ExtractedField{
				Value:      formatNumber(value),
				Confidence: 0.85,
				Source:     "json-ld",
				PageSource: pageCategory,
			})
		}
	}
	if employees, ok := data["numberOfEmployees"].(float64); ok {
		updateFieldIfBetter(&output.Employees, ExtractedField{
			Value:      formatNumber(employees),
			Confidence: 0.85,
			Source:     "json-ld",
			PageSource: pageCategory,
		})
	}
}

// === BUSINESS METRICS EXTRACTION ===

// extractBusinessMetrics extracts business metrics including employee count, turnover, and ATECO code.
// These fields typically have low confidence (0.40-0.50) as they are rarely published on company websites.
// Extracts using regex patterns on visible text content.
func extractBusinessMetrics(ctx *extractionContext, page *pageData, output *Output) {
	// Extract employee count using regex
	for _, pattern := range employeesPatterns {
		if matches := pattern.FindStringSubmatch(page.text); len(matches) > 1 {
			employees := normalizeNumber(matches[1])
			updateFieldIfBetter(&output.Employees, ExtractedField{
				Value:      employees,
				Confidence: 0.50,
				Source:     "regex",
				PageSource: page.category,
			})
			ctx.sourcesUsed["regex"] = true
			break
		}
	}

	// Extract turnover/revenue
	for _, pattern := range turnoverPatterns {
		if matches := pattern.FindStringSubmatch(page.text); len(matches) > 1 {
			turnover := matches[1]
			updateFieldIfBetter(&output.YearlyTurnover, ExtractedField{
				Value:      turnover,
				Confidence: 0.40,
				Source:     "regex",
				PageSource: page.category,
			})
			ctx.sourcesUsed["regex"] = true
			break
		}
	}

	// Extract ATECO code
	if matches := atecoPattern.FindStringSubmatch(page.text); len(matches) > 1 {
		ateco := matches[1]
		updateFieldIfBetter(&output.AtecoCode, ExtractedField{
			Value:      ateco,
			Confidence: 0.80,
			Source:     "regex",
			PageSource: page.category,
		})
		ctx.sourcesUsed["regex"] = true
	}
}

// === PRODUCT DESCRIPTION EXTRACTION ===

// extractProductDescription extracts descriptions of products or services offered.
// It uses JSON-LD data, meta description tags, and Open Graph tags.
// Only matches descriptions containing product or service-related keywords.
func extractProductDescription(ctx *extractionContext, page *pageData, output *Output) {
	// Check JSON-LD for product/service descriptions
	for _, jsonData := range page.jsonLD {
		if desc := extractJSONLDProductDescription(jsonData); desc != "" {
			updateFieldIfBetter(&output.ProductDescription, ExtractedField{
				Value:      desc,
				Confidence: 0.85,
				Source:     "json-ld",
				PageSource: page.category,
			})
			ctx.sourcesUsed["json-ld"] = true
		}
	}

	// Check meta description (often contains product/service info)
	if desc := page.metaTags["description"]; desc != "" {
		// Check if it contains product/service keywords
		keywords := []string{"prodotti", "servizi", "products", "services", "offriamo", "forniamo", "offre", "offer", "provide"}
		for _, kw := range keywords {
			if strings.Contains(strings.ToLower(desc), kw) {
				updateFieldIfBetter(&output.ProductDescription, ExtractedField{
					Value:      desc,
					Confidence: 0.60,
					Source:     "meta-tag",
					PageSource: page.category,
				})
				ctx.sourcesUsed["meta-tag"] = true
				break
			}
		}
	}

	// Try to extract from og:description
	if desc := page.metaTags["og:description"]; desc != "" {
		updateFieldIfBetter(&output.ProductDescription, ExtractedField{
			Value:      desc,
			Confidence: 0.55,
			Source:     "og:tag",
			PageSource: page.category,
		})
		ctx.sourcesUsed["og:tag"] = true
	}
}

// extractJSONLDProductDescription extracts product or service description from JSON-LD structured data.
// It looks for Product or Service types and extracts descriptions.
// Also checks makesOffer fields for service descriptions.
// Returns the description string or empty string if not found.
func extractJSONLDProductDescription(data map[string]interface{}) string {
	// Check for Product type
	typeVal, ok := data["@type"].(string)
	if ok && (strings.EqualFold(typeVal, "Product") || strings.EqualFold(typeVal, "Service")) {
		if desc, ok := data["description"].(string); ok {
			return desc
		}
	}

	// Check for offers description
	if offers, ok := data["makesOffer"].([]interface{}); ok && len(offers) > 0 {
		if offer, ok := offers[0].(map[string]interface{}); ok {
			if desc, ok := offer["description"].(string); ok {
				return desc
			}
		}
	}

	return ""
}

// === HELPER FUNCTIONS ===

// getEmailConfidenceByPrefix adjusts email confidence based on the local part prefix.
// It boosts confidence for generic contact emails (info@, contact@) and reduces it for
// department-specific addresses (admin@, billing@, legal@). Returns adjusted confidence.
func getEmailConfidenceByPrefix(email string, baseConfidence float64) float64 {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return baseConfidence
	}

	prefix := strings.ToLower(parts[0])

	// High-priority prefixes (general contact emails)
	highPriority := []string{"info", "contact", "contatti", "contatto"}
	for _, p := range highPriority {
		if prefix == p {
			return baseConfidence + 0.08 // Boost for preferred contact emails
		}
	}

	// Medium-priority prefixes (sales/support)
	mediumPriority := []string{"sales", "vendite", "support", "supporto", "help", "assistenza", "commerciale"}
	for _, p := range mediumPriority {
		if prefix == p {
			return baseConfidence + 0.03
		}
	}

	// Low-priority prefixes (often PEC or internal, less suitable as main contact)
	// Note: PEC emails are already filtered separately, but some might slip through
	lowPriority := []string{"amministrazione", "admin", "fatturazione", "billing", "legale", "legal", "hr", "risorse"}
	for _, p := range lowPriority {
		if prefix == p {
			return baseConfidence - 0.10 // Reduce priority
		}
	}

	return baseConfidence
}

// isLikelyVATNotPhone determines if a number that was extracted as a potential phone
// is more likely to be an Italian VAT number (Partita IVA).
// It returns true if the number has exactly 11 digits with no international prefix or separators.
// Italian VAT numbers are typically continuous 11-digit numbers unlike formatted phone numbers.
func isLikelyVATNotPhone(originalPhone, cleanedDigits string) bool {
	// Only consider numbers that are exactly 11 digits (Italian VAT length)
	if len(cleanedDigits) != 11 {
		return false
	}

	trimmed := strings.TrimSpace(originalPhone)

	// If the original has a + prefix, it's likely a phone with international code
	if strings.HasPrefix(trimmed, "+") {
		return false
	}

	// If starts with 00 followed by country code (e.g., 0039 for Italy), it's a phone
	// But 00 at start without being a full international prefix could be VAT
	// Italian international prefix is 0039, so check for that specific pattern
	if strings.HasPrefix(trimmed, "0039") || strings.HasPrefix(trimmed, "00 39") || strings.HasPrefix(trimmed, "00-39") {
		return false
	}

	// Italian VAT numbers start with 0 (for domestic companies) or other digits
	// but phone numbers with area codes in Italy start with 0 too (e.g., 02 for Milan)
	// The key difference: VAT is a continuous 11-digit number, phones have separators

	// Check if the original has no separators (spaces, dashes, dots) - likely VAT
	// Phone numbers typically have formatting like: 02 1234567, 02-123-4567
	separatorCount := 0
	for _, c := range originalPhone {
		if c == ' ' || c == '-' || c == '.' || c == '(' || c == ')' {
			separatorCount++
		}
	}

	// If no separators and exactly 11 digits, very likely VAT
	if separatorCount == 0 {
		return true
	}

	// If only 1 separator and 11 digits, could still be VAT (e.g., "003 43170510")
	// But be more conservative here - if there's any separator, likely phone
	// unless it's clearly a VAT pattern

	return false
}

// formatNumber formats a float64 as a clean integer string with no decimals or separators.
// It truncates decimals and returns the result as a string.
func formatNumber(n float64) string {
	// Convert to integer (truncate decimals)
	intVal := int64(n)
	if intVal < 0 {
		intVal = -intVal
	}

	// Build the result string
	if intVal == 0 {
		return "0"
	}

	var result strings.Builder
	for intVal > 0 {
		digit := intVal % 10
		result.WriteByte(byte('0' + digit))
		intVal /= 10
	}

	// Reverse the string
	s := result.String()
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	return string(runes)
}

// normalizeNumber extracts and normalizes a number from a string.
// It removes thousands separators and normalizes decimal separators.
// Returns the cleaned number string.
func normalizeNumber(s string) string {
	// Remove thousands separators and normalize decimal separators
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ".", "")  // Remove thousands separator
	s = strings.ReplaceAll(s, ",", ".") // Normalize decimal separator
	s = regexp.MustCompile(`[^\d.]`).ReplaceAllString(s, "")

	// Remove trailing decimals for whole numbers
	if strings.Contains(s, ".") {
		parts := strings.Split(s, ".")
		if len(parts) == 2 && (parts[1] == "0" || parts[1] == "00") {
			return parts[0]
		}
	}

	return s
}
