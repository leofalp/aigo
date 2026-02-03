# SiteDataExtractor Tool

A deterministic tool for extracting structured company data from HTML pages. This tool performs pattern-based extraction without using LLMs, making it fast, free, and predictable.

## Purpose

The SiteDataExtractor analyzes HTML content from company websites and extracts structured business information including:

- **Identity**: Company name, VAT number, Tax code
- **Location**: Address, City, ZIP, Province, Country
- **Contact**: Email, Phone, Fax, PEC (Italian certified email)
- **Online Presence**: Logo URL, Social media links (Facebook, LinkedIn, Twitter, Instagram, YouTube)
- **Business Data**: ATECO code, Employee count, Turnover, Industry, Description

## Input/Output

### Input

```go
type Input struct {
    // SiteStructure contains the output from urlextractor including base URL and standard pages.
    SiteStructure urlextractor.Output `json:"site_structure"`

    // Pages contains the HTML content of fetched pages, keyed by page category.
    // Example: {"home": "<html>...", "contact": "<html>...", "about": "<html>..."}
    Pages map[string]string `json:"pages"`
}
```

### Output

```go
type Output struct {
    // Identity fields
    CompanyName ExtractedField `json:"company_name"`
    VAT         ExtractedField `json:"vat"`
    TaxCode     ExtractedField `json:"tax_code"`

    // Location fields
    Address      ExtractedField `json:"address"`
    ZipCode      ExtractedField `json:"zip_code"`
    City         ExtractedField `json:"city"`
    ProvinceCode ExtractedField `json:"province_code"`
    Region       ExtractedField `json:"region"`
    CountryCode  ExtractedField `json:"country_code"`
    State        ExtractedField `json:"state"`

    // Contact fields
    Phone ExtractedField `json:"phone"`
    Fax   ExtractedField `json:"fax"`
    Email ExtractedField `json:"email"`
    PEC   ExtractedField `json:"pec"`

    // Online presence fields
    Website    ExtractedField `json:"website"`
    LogoURL    ExtractedField `json:"logo_url"`
    Facebook   ExtractedField `json:"facebook"`
    LinkedIn   ExtractedField `json:"linkedin"`
    LinkedInID ExtractedField `json:"linkedin_id"`
    Twitter    ExtractedField `json:"twitter"`
    Instagram  ExtractedField `json:"instagram"`
    YouTube    ExtractedField `json:"youtube"`

    // Business data fields
    AtecoCode          ExtractedField `json:"ateco_code"`
    Employees          ExtractedField `json:"employees"`
    YearlyTurnover     ExtractedField `json:"yearly_turnover"`
    DateOfBalance      ExtractedField `json:"date_of_balance"`
    Industry           ExtractedField `json:"industry"`
    Description        ExtractedField `json:"description"`
    ProductDescription ExtractedField `json:"product_description"`

    // Metadata
    OverallConfidence float64  `json:"overall_confidence"` // 0-1, weighted average
    FieldsExtracted   int      `json:"fields_extracted"`   // Count of non-empty fields
    FieldsTotal       int      `json:"fields_total"`       // Total extractable fields
    SourcesUsed       []string `json:"sources_used"`       // Extraction methods used
}

type ExtractedField struct {
    Value      string   `json:"value"`                 // Extracted value
    Confidence float64  `json:"confidence"`            // 0-1 confidence score
    Source     string   `json:"source"`                // Extraction method
    PageSource string   `json:"page_source"`           // Page category where found
    Candidates []string `json:"candidates,omitempty"`  // Alternative values
}
```

## Extraction Methods and Confidence Levels

### Logo Extraction

| Method | Confidence | Description |
|--------|------------|-------------|
| JSON-LD `Organization.logo` | 0.95 | Schema.org structured data |
| `<meta property="og:image">` (same domain) | 0.85 | Open Graph image |
| `<link rel="apple-touch-icon">` | 0.80 | iOS icon (high quality) |
| `<link rel="icon" type="image/svg+xml">` | 0.78 | SVG favicon |
| `<link rel="icon" type="image/png">` | 0.75 | PNG favicon |
| `<img>` with logo class/id | 0.60 | DOM heuristic |
| URL pattern `/logo.*` | 0.50 | Filename pattern |
| `favicon.ico` | 0.30 | Fallback |

### Contact Extraction

| Field | Pattern | Confidence |
|-------|---------|------------|
| Email (JSON-LD) | Schema.org `email` | 0.95 |
| Email (mailto: link) | `href="mailto:..."` | 0.95 |
| Email (same domain) | Regex in text | 0.90 |
| Email (other) | Regex in text | 0.70 |
| PEC | `@pec.it`, `@legalmail.it`, etc. | 0.85-0.90 |
| Phone (tel: link) | `href="tel:..."` | 0.90 |
| Phone (JSON-LD) | Schema.org `telephone` | 0.95 |
| Phone (text) | International patterns | 0.75 |
| Fax | "Fax:" keyword + number | 0.80 |

### Social Media Extraction

| Network | Pattern |
|---------|---------|
| Facebook | `facebook.com/` (excludes share links) |
| LinkedIn | `linkedin.com/company/` |
| Twitter/X | `twitter.com/` or `x.com/` (excludes intent) |
| Instagram | `instagram.com/` (excludes explore/p/) |
| YouTube | `youtube.com/channel/`, `/c/`, `/user/`, `/@` |

Sources: JSON-LD `sameAs` (0.95), HTML links (0.85)

### Address Extraction

| Field | Source | Confidence |
|-------|--------|------------|
| Full address | JSON-LD `PostalAddress` | 0.95 |
| Street | Regex (Via, Street, Rue, etc.) | 0.65 |
| ZIP code | Country-specific patterns | 0.70 |
| Province | Italian 2-letter codes | 0.50 |
| Country | ISO codes or names | 0.60-0.95 |

### Company Data Extraction

| Field | Source | Confidence |
|-------|--------|------------|
| Name (JSON-LD `legalName`) | Schema.org | 0.98 |
| Name (JSON-LD `name`) | Schema.org | 0.95 |
| Name (og:site_name) | Meta tag | 0.85 |
| VAT (P.IVA pattern) | Regex | 0.90 |
| VAT (JSON-LD `vatID`) | Schema.org | 0.95 |
| Tax Code (C.F. pattern) | Regex (16 chars) | 0.70-0.90 |
| Description | JSON-LD or meta | 0.85-0.95 |

### Business Metrics (Low Confidence)

These fields are rarely found on company websites:

| Field | Pattern | Confidence |
|-------|---------|------------|
| Employees | "X dipendenti/employees" | 0.50 |
| Turnover | "fatturato/revenue" + amount | 0.40 |
| ATECO Code | "ATECO: XX.XX.XX" | 0.80 |

## Usage Example

```go
package main

import (
    "context"
    "fmt"

    "github.com/leofalp/aigo/providers/tool/sitedataextractor"
    "github.com/leofalp/aigo/providers/tool/urlextractor"
)

func main() {
    // Assume you've fetched pages with webfetch
    input := sitedataextractor.Input{
        SiteStructure: urlextractor.Output{
            BaseURL: "https://www.example.com",
        },
        Pages: map[string]string{
            "home":    "<html>...</html>",
            "contact": "<html>...</html>",
            "about":   "<html>...</html>",
        },
    }

    output, err := sitedataextractor.Extract(context.Background(), input)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Company: %s (confidence: %.2f)\n",
        output.CompanyName.Value, output.CompanyName.Confidence)
    fmt.Printf("Email: %s\n", output.Email.Value)
    fmt.Printf("Phone: %s\n", output.Phone.Value)
    fmt.Printf("Overall confidence: %.2f\n", output.OverallConfidence)
    fmt.Printf("Fields extracted: %d/%d\n",
        output.FieldsExtracted, output.FieldsTotal)
}
```

## Limitations

1. **JavaScript-rendered content**: The tool parses static HTML only. Websites that render content via JavaScript may yield incomplete results.

2. **Non-standard formats**: Unusual address formats, phone number styles, or custom HTML structures may not be recognized.

3. **Business metrics**: Fields like employee count, turnover, and ATECO codes are rarely present on company websites and typically require external data sources (business registries, LinkedIn, etc.).

4. **Language**: Patterns are optimized for European languages (Italian, English, German, French, Spanish). Other languages may have lower extraction rates.

5. **PEC validation**: The tool identifies likely PEC addresses by domain pattern but doesn't verify they are actually certified.

## Cost

This tool is free - it performs local HTML parsing without any external API calls.

- **Amount**: $0.00
- **Average Duration**: ~50ms per extraction
- **Accuracy**: ~80% (varies by page structure)
