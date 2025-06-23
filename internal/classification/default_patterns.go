package classification

// DefaultPatterns returns the default set of transaction classification patterns.
func DefaultPatterns() []Pattern {
	return []Pattern{
		// Income patterns - highest priority
		{
			Name:       "Direct Deposit",
			Type:       PatternTypeIncome,
			Regex:      `\b(DIRECTDEP|DIRECT\s*DEP|DIR\s*DEP|DD|PAYROLL|SALARY|WAGES)\b`,
			Priority:   100,
			Confidence: 0.95,
		},
		{
			Name:       "Interest Income",
			Type:       PatternTypeIncome,
			Regex:      `\b(INTEREST|INT\s*EARNED|INT\s*INCOME|DIVIDEND|DIV)\b`,
			Priority:   95,
			Confidence: 0.90,
		},
		{
			Name:       "Credit/Refund",
			Type:       PatternTypeIncome,
			Regex:      `\b(CREDIT|REFUND|REIMB|REIMBURSEMENT|CASHBACK|CASH\s*BACK)\b`,
			Priority:   90,
			Confidence: 0.85,
		},
		{
			Name:       "Tax Refund",
			Type:       PatternTypeIncome,
			Regex:      `\b(TAX\s*REF|IRS\s*TREAS|STATE\s*TAX\s*REF|FED\s*TAX\s*REF)\b`,
			Priority:   95,
			Confidence: 0.95,
		},
		{
			Name:       "Vendor Payment",
			Type:       PatternTypeIncome,
			Regex:      `\b(PAYMENT\s*FROM|INVOICE|CLIENT|CUSTOMER\s*PAY)\b`,
			Priority:   85,
			Confidence: 0.80,
		},
		{
			Name:       "Investment Income",
			Type:       PatternTypeIncome,
			Regex:      `\b(CAPITAL\s*GAIN|STOCK\s*SALE|INVESTMENT\s*INCOME|401K\s*DIST)\b`,
			Priority:   90,
			Confidence: 0.85,
		},
		{
			Name:       "Rental Income",
			Type:       PatternTypeIncome,
			Regex:      `\b(RENT\s*INCOME|RENTAL\s*PAYMENT|TENANT)\b`,
			Priority:   85,
			Confidence: 0.85,
		},
		{
			Name:       "Social Security",
			Type:       PatternTypeIncome,
			Regex:      `\b(SOC\s*SEC|SOCIAL\s*SECURITY|SSA\s*TREAS)\b`,
			Priority:   95,
			Confidence: 0.95,
		},
		{
			Name:       "Pension",
			Type:       PatternTypeIncome,
			Regex:      `\b(PENSION|RETIREMENT\s*INCOME|ANNUITY)\b`,
			Priority:   90,
			Confidence: 0.90,
		},
		{
			Name:       "Bonus",
			Type:       PatternTypeIncome,
			Regex:      `\b(BONUS|COMMISSION|INCENTIVE|PERFORMANCE\s*PAY)\b`,
			Priority:   85,
			Confidence: 0.85,
		},

		// Transfer patterns
		{
			Name:       "Account Transfer",
			Type:       PatternTypeTransfer,
			Regex:      `\b(TRANSFER|XFER|TFR|MOVE\s*MONEY|ACCOUNT\s*TO\s*ACCOUNT)\b`,
			Priority:   80,
			Confidence: 0.85,
		},
		{
			Name:       "Wire Transfer",
			Type:       PatternTypeTransfer,
			Regex:      `\b(WIRE\s*IN|WIRE\s*OUT|WIRE\s*TRANSFER|WIRE\s*XFER)\b`,
			Priority:   85,
			Confidence: 0.90,
		},
		{
			Name:       "Investment Transfer",
			Type:       PatternTypeTransfer,
			Regex:      `\b(401K|IRA|ROTH|BROKERAGE)\s*(CONTRIBUTION|TRANSFER|ROLLOVER)\b`,
			Priority:   80,
			Confidence: 0.85,
		},
		{
			Name:       "Savings Transfer",
			Type:       PatternTypeTransfer,
			Regex:      `\b(TO\s*SAVINGS|FROM\s*SAVINGS|SAVINGS\s*TRANSFER)\b`,
			Priority:   75,
			Confidence: 0.80,
		},
		{
			Name:       "Credit Card Payment",
			Type:       PatternTypeTransfer,
			Regex:      `\b(CC\s*PAYMENT|CREDIT\s*CARD\s*PAY|CARD\s*PAYMENT|PMT\s*TO)\b`,
			Priority:   75,
			Confidence: 0.80,
		},
		{
			Name:       "Loan Payment Transfer",
			Type:       PatternTypeTransfer,
			Regex:      `\b(LOAN\s*PMT|MORTGAGE\s*PMT|AUTO\s*PMT|STUDENT\s*LOAN\s*PMT)\b`,
			Priority:   70,
			Confidence: 0.75,
		},

		// Common expense patterns (lower priority)
		{
			Name:       "ATM Withdrawal",
			Type:       PatternTypeExpense,
			Regex:      `\b(ATM|CASH\s*WITHDRAWAL|WITHDRAW)\b`,
			Priority:   50,
			Confidence: 0.80,
		},
		{
			Name:       "Purchase",
			Type:       PatternTypeExpense,
			Regex:      `\b(PURCHASE|POS|DEBIT|CARD\s*PURCHASE)\b`,
			Priority:   40,
			Confidence: 0.70,
		},
		{
			Name:       "Fee",
			Type:       PatternTypeExpense,
			Regex:      `\b(FEE|CHARGE|SERVICE\s*CHG|PENALTY)\b`,
			Priority:   45,
			Confidence: 0.75,
		},
		{
			Name:       "Bill Payment",
			Type:       PatternTypeExpense,
			Regex:      `\b(BILL\s*PAY|AUTOPAY|RECURRING|SUBSCRIPTION)\b`,
			Priority:   45,
			Confidence: 0.70,
		},
	}
}
