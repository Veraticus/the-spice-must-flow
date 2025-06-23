# Comprehensive TUI Design for the-spice-must-flow

## Vision

Transform the-spice-must-flow from a CLI-first application into a delightful TUI-first experience that makes personal finance management feel like a video game rather than a chore. The TUI should be so intuitive and efficient that users prefer it over any web-based finance app.

## Core Design Principles

1. **Keyboard-First**: Every action accessible via keyboard shortcuts
2. **Visual Feedback**: Immediate visual response to every action  
3. **Progressive Disclosure**: Show complexity only when needed
4. **Contextual Help**: Always know what keys do what
5. **Delightful Interactions**: Make finance management fun

## Navigation Structure

```
┌─────────────────────────────────────────────────────────────┐
│ [D]ashboard  [T]ransactions  [A]nalytics  [M]anage  [S]ettings │
└─────────────────────────────────────────────────────────────┘

Each mode accessible via:
- Tab key to cycle through
- Direct letter key (D, T, A, M, S)
- Number keys (1-5)
```

## Mode Designs

### 1. Dashboard Mode (Home)

```
╔══════════════════════════════════════════════════════════════╗
║ 💰 the-spice-must-flow                          [?] Help [Q] ║
╟──────────────────────────────────────────────────────────────╢
║                                                              ║
║  📊 Quick Stats               🎯 Actions                     ║
║  ├─ Total Balance: $12,453    ├─ [I] Import new             ║
║  ├─ This Month: -$2,341       ├─ [C] Classify (42)          ║
║  ├─ Uncategorized: 42         ├─ [F] View flow              ║
║  └─ Last Import: 2 days ago   └─ [E] Export report          ║
║                                                              ║
║  📈 Spending Trend (Last 30 days)                            ║
║  ┌────────────────────────────────────────────┐             ║
║  │     ▁▃▅▇█▅▃▁  ▃▅▇  ▁▃▅    ▇█▅▃▁            │             ║
║  └────────────────────────────────────────────┘             ║
║                                                              ║
║  🔔 Alerts                                                   ║
║  ⚠️  42 transactions need categorization                     ║
║  ℹ️  New statement available from Chase                      ║
║  ✅ Export to Google Sheets completed                        ║
║                                                              ║
║  📝 Recent Activity                                          ║
║  ├─ Classified 15 transactions (5 min ago)                   ║
║  ├─ Imported 73 transactions from Chase (2 hours ago)        ║
║  └─ Created checkpoint "pre-june-import" (3 hours ago)       ║
║                                                              ║
╚══════════════════════════════════════════════════════════════╝
```

### 2. Transactions Mode (Unified Workflow)

```
╔══════════════════════════════════════════════════════════════╗
║ 📑 Transactions                       [/] Search [F] Filter   ║
╟──────────────────────────────────────────────────────────────╢
║ ┌─ Filters ─────────────────────┐ ┌─ Quick Stats ──────────┐ ║
║ │ Period: Last 30 days      [P] │ │ Showing: 234 of 1,052  │ ║
║ │ Status: Uncategorized     [S] │ │ Selected: 0             │ ║
║ │ Account: All              [A] │ │ Total: -$4,521.32       │ ║
║ └────────────────────────────────┘ └─────────────────────────┘ ║
║                                                              ║
║  Date     Merchant          Amount   Category    Account  St ║
║ ─────────────────────────────────────────────────────────────║
║ ▶ Jun 22  Whole Foods      -123.45  [········]   Chase    ? ║
║   Jun 22  Netflix           -15.99  Entertain.   Chase    ✓ ║
║   Jun 21  Shell Gas         -45.67  [········]   Chase    ? ║
║   Jun 21  Target          -234.56  [········]   Chase    ? ║
║                                                              ║
║ ┌─ Actions ──────────────────────────────────────────────────┐ ║
║ │ [Enter] Classify  [V] Multi-select  [B] Batch  [I] Import │ ║
║ └────────────────────────────────────────────────────────────┘ ║
╚══════════════════════════════════════════════════════════════╝
```

### 3. Analytics Mode (Enhanced Flow)

```
╔══════════════════════════════════════════════════════════════╗
║ 📊 Analytics                    Jun 2024  [←] Previous [→]   ║
╟──────────────────────────────────────────────────────────────╢
║                                                              ║
║  Income vs Expenses          Category Breakdown              ║
║  ┌──────────────────┐       ┌─────────────────────────┐    ║
║  │ ▓▓▓▓▓▓▓▓ +5,234 │       │ 🥬 Groceries    $823 ▓▓▓▓│    ║
║  │ ████████ -3,122 │       │ 🏠 Home        $1200 ████│    ║
║  └──────────────────┘       │ 🚗 Transport    $456 ▓▓  │    ║
║  Net: +$2,112               │ 🍕 Dining       $234 ▓   │    ║
║                             └─────────────────────────┘    ║
║                                                              ║
║  Trend Analysis                                              ║
║  ┌────────────────────────────────────────────────────┐     ║
║  │  4k ┤                      ╭─╮                      │     ║
║  │  3k ┤           ╭─╮       ╱  ╲    ╭─╮              │     ║
║  │  2k ┤     ╭─────╯  ╲─────╯    ╲──╯  ╲             │     ║
║  │  1k ┤────╯                            ╲            │     ║
║  │   0 └──┬───┬───┬───┬───┬───┬───┬───┬──┴─┬───┬─────│     ║
║  │      Jan Feb Mar Apr May Jun Jul Aug Sep Oct Nov    │     ║
║  └────────────────────────────────────────────────────┘     ║
║                                                              ║
║  [E] Export  [D] Drill-down  [C] Compare periods            ║
╚══════════════════════════════════════════════════════════════╝
```

### 4. Management Mode (Hub)

```
╔══════════════════════════════════════════════════════════════╗
║ ⚙️  Management          [C]ategories  [V]endors  [P]atterns  ║
╟──────────────────────────────────────────────────────────────╢
║                                                              ║
║  Categories                                    Usage  Actions ║
║ ─────────────────────────────────────────────────────────────║
║  🥬 Groceries      Food and household items     234  [E][M] ║
║  🏠 Home           Rent, utilities, repairs      45  [E][M] ║
║  🚗 Transportation Gas, uber, maintenance        122  [E][M] ║
║  🍕 Dining Out     Restaurants and takeout       89  [E][M] ║
║  + Add new category...                               [A]    ║
║                                                              ║
║  Quick Actions                                               ║
║  ├─ [A] Add new      ├─ [E] Edit                            ║
║  ├─ [M] Merge        └─ [D] Delete                          ║
║  └─ [/] Search                                               ║
║                                                              ║
║  💡 Tips                                                     ║
║  • Press number keys 1-9 to quickly jump to categories       ║
║  • Use Tab to switch between Categories/Vendors/Patterns     ║
║  • Drag categories to reorder (or use Shift+↑/↓)            ║
╚══════════════════════════════════════════════════════════════╝
```

### 5. Settings Mode

```
╔══════════════════════════════════════════════════════════════╗
║ ⚙️  Settings                                                 ║
╟──────────────────────────────────────────────────────────────╢
║                                                              ║
║  [C] Connections          [B] Backups                       ║
║  ├─ Plaid: Connected      ├─ Auto-backup: Enabled           ║
║  ├─ Sheets: Connected     ├─ Last backup: 2 hours ago       ║
║  └─ SimpleFIN: Not set    └─ Storage used: 45 MB            ║
║                                                              ║
║  [T] Theme               [K] Keyboard Shortcuts              ║
║  ├─ Current: Default     ├─ Style: Vim                      ║
║  └─ Options: 3 themes    └─ [View all shortcuts]            ║
║                                                              ║
║  [D] Data Management                                         ║
║  ├─ Database: ~/spice.db (45 MB)                           ║
║  ├─ Checkpoints: 12 saved                                   ║
║  └─ [Manage checkpoints]                                     ║
║                                                              ║
║  [A] About                                                   ║
║  the-spice-must-flow v1.2.0                                 ║
║  A delightful finance categorization engine                  ║
╚══════════════════════════════════════════════════════════════╝
```

## Key Interactions

### Global Shortcuts (Available everywhere)
- `?` - Context-sensitive help
- `q` - Quit to previous mode (or exit if at root)
- `Q` - Quit application
- `/` - Global search
- `Ctrl+S` - Save/Sync
- `Tab` - Next main mode
- `Shift+Tab` - Previous main mode
- `1-5` - Jump to mode by number

### Transaction Selection
- `j/k` or `↓/↑` - Navigate
- `Space` - Toggle selection
- `v` - Enter visual mode
- `V` - Select all visible
- `Enter` - Classify/Edit
- `Shift+Enter` - Quick classify with last category

### Quick Classification
- `1-9` - Select category by number
- `0` - Custom category
- `s` - Skip
- `u` - Undo last
- `a` - Accept AI suggestion

### Batch Operations
- `Ctrl+A` - Select all
- `Ctrl+D` - Deselect all
- `b` - Batch classify selected
- `m` - Move to category
- `d` - Delete (with confirmation)

## Visual Enhancements

### Color Coding
- 🟢 Green - Income/Positive
- 🔴 Red - Expenses/Negative  
- 🟡 Yellow - Needs attention
- 🔵 Blue - Informational
- ⚫ Gray - Disabled/Muted

### Status Indicators
- `✓` - Categorized
- `?` - Uncategorized
- `!` - Needs review
- `⏳` - Processing
- `🔄` - Syncing

### Progress Indicators
- Animated spinners for async operations
- Progress bars for batch operations
- Sparklines for trends
- Live counters during classification

## Implementation Strategy

### Phase 1: Navigation Framework
1. Create mode enum and state machine
2. Implement global navigation
3. Add keyboard shortcut system
4. Create help system

### Phase 2: Core Workflows
1. Unified transaction view
2. Inline classification
3. Batch operations
4. Search and filter

### Phase 3: Visualizations
1. Dashboard with stats
2. Analytics charts
3. Spending trends
4. Category breakdowns

### Phase 4: Management
1. Category CRUD
2. Vendor rules
3. Pattern testing
4. Settings UI

### Phase 5: Polish
1. Animations
2. Themes
3. Customization
4. Performance optimization

## Technical Considerations

### State Management
- Central state store for all modes
- Mode-specific substates
- Undo/redo stack
- Persistent UI preferences

### Performance
- Virtual scrolling for large datasets
- Lazy loading for charts
- Background data fetching
- Debounced search

### Data Flow
```
User Input → Mode Handler → State Update → UI Render
     ↑                                          ↓
     └──────── Side Effects (API, DB) ←─────────┘
```

### Testing Strategy
- Mode transition tests
- Keyboard shortcut tests
- State management tests
- Visual regression tests
- Integration tests with real data

## Success Metrics

1. **Efficiency**: Classify 100 transactions in < 5 minutes
2. **Discoverability**: New users productive in < 10 minutes
3. **Delight**: Users prefer TUI over web alternatives
4. **Reliability**: Zero data loss, instant saves
5. **Performance**: < 50ms response to any action

## Future Enhancements

1. **AI Assistant Mode**: Natural language commands
2. **Forecasting View**: Predict future spending
3. **Goals Tracking**: Budget vs actual
4. **Multi-Account Dashboard**: See all accounts at once
5. **Plugins**: User-defined visualizations and workflows

This comprehensive TUI will transform the-spice-must-flow into the most delightful personal finance tool available, making money management feel less like work and more like play.