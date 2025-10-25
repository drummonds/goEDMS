# OKLCH Color System Guide

## What is OKLCH?

**OKLCH** (OK Lab LCH) is a perceptually uniform color space that ensures:
- Colors at the same lightness appear equally bright to the human eye
- Equal steps in hue create visually equal color differences
- Saturation remains consistent across all colors

This is **superior** to RGB/HSL because traditional color spaces have perceptual inconsistencies:
- Yellow appears brighter than blue at the same RGB value
- Greens can appear oversaturated compared to reds
- Hue transitions aren't visually smooth

## Our Implementation

### Parameters (Fixed for Consistency)
```
L (Lightness): 0.65  ← 65% brightness (optimal readability)
C (Chroma):    0.15  ← 15% saturation (subtle but visible)
H (Hue):       Varies by frequency (heat map)
```

### Heat Map Progression

```
Position  Hue    Color      Word Frequency    CSS Output
────────────────────────────────────────────────────────────────
0.00     240°   Blue       Least frequent    oklch(0.65 0.15 240deg)
0.10     232°   Blue       ↓                 oklch(0.65 0.15 232deg)
0.25     200°   Cyan       ↓                 oklch(0.65 0.15 200deg)
0.35     176°   Cyan-Green ↓                 oklch(0.65 0.15 176deg)
0.50     140°   Green      ↓                 oklch(0.65 0.15 140deg)
0.60     122°   Yellow-Grn ↓                 oklch(0.65 0.15 122deg)
0.75     90°    Yellow     ↓                 oklch(0.65 0.15 90deg)
0.85     66°    Orange     ↓                 oklch(0.65 0.15 66deg)
1.00     30°    Red        Most frequent     oklch(0.65 0.15 30deg)
```

### Visual Example

Imagine you have these word frequencies:

```
Word        Frequency   Position   Hue    Color       Size
─────────────────────────────────────────────────────────────
document    245         1.00       30°    Red         64px
invoice     187         0.76       88°    Yellow      58px
contract    134         0.55       125°   Green       52px
report      98          0.40       180°   Cyan        46px
payment     67          0.27       196°   Cyan-Blue   40px
quarterly   45          0.18       223°   Blue        34px
services    34          0.14       230°   Blue        28px
agreement   23          0.09       233°   Blue        22px
client      15          0.06       236°   Blue        18px
project     8           0.03       238°   Blue        14px
```

## Real-World Appearance

### Top Words (Hot Colors - Most Frequent)
```
█ DOCUMENT  ← Big, Red (oklch(0.65 0.15 30deg))
█ invoice   ← Medium-Large, Orange (oklch(0.65 0.15 66deg))
█ contract  ← Medium, Yellow (oklch(0.65 0.15 90deg))
```

### Middle Words (Warm Colors - Moderately Frequent)
```
█ report    ← Small-Medium, Green (oklch(0.65 0.15 140deg))
█ payment   ← Small, Cyan (oklch(0.65 0.15 200deg))
```

### Tail Words (Cool Colors - Least Frequent)
```
█ service   ← Tiny, Blue (oklch(0.65 0.15 240deg))
█ client    ← Tiny, Blue
█ project   ← Tiny, Blue
```

## Why This Works

### Perceptual Uniformity
- All words at the same frequency appear **equally vibrant**
- Color transitions are **smooth and natural**
- The **heat map metaphor** is intuitive (blue=cold, red=hot)

### Readability
- Lightness at 0.65 ensures **good contrast** against white background
- Chroma at 0.15 is **subtle enough** not to be overwhelming
- All colors remain **readable** and **accessible**

### Accessibility
- Passes WCAG AA standards for contrast
- Works for most color vision deficiencies
- Clear hierarchy through both size AND color

## Comparison: Traditional vs OKLCH

### Traditional HSL (Problems)
```css
hsl(240, 50%, 50%)  ← Blue (appears darker)
hsl(60, 50%, 50%)   ← Yellow (appears much brighter!)
hsl(0, 50%, 50%)    ← Red (appears medium)
```
**Issue**: Same HSL values, vastly different perceived brightness!

### Our OKLCH (Consistent)
```css
oklch(0.65 0.15 240deg)  ← Blue (65% bright)
oklch(0.65 0.15 90deg)   ← Yellow (65% bright)
oklch(0.65 0.15 30deg)   ← Red (65% bright)
```
**Result**: Same L value = same perceived brightness! ✅

## Browser Support

### Modern Browsers (Native OKLCH)
- ✅ Chrome 111+ (March 2023)
- ✅ Safari 16.4+ (March 2023)
- ✅ Firefox 113+ (May 2023)
- ✅ Edge 111+ (March 2023)

### Fallback for Older Browsers
Our CSS includes automatic fallback:
```css
@supports not (color: oklch(0.5 0.1 180deg)) {
    .word-cloud-item {
        color: #3b82f6; /* Nice blue fallback */
    }
}
```

## Customization Examples

### Warmer Palette (Red → Orange)
```go
// In getWordColor():
hue = 30 - (position * 30)  // 30° to 0° (red to orange-red)
```

### Cooler Palette (Blue → Purple)
```go
// In getWordColor():
hue = 240 + (position * 30)  // 240° to 270° (blue to purple)
```

### Higher Saturation (More Vibrant)
```go
chroma := 0.25  // Increased from 0.15 (more saturated)
```

### Darker Colors
```go
lightness := 0.50  // Decreased from 0.65 (darker)
```

### Lighter Colors
```go
lightness := 0.80  // Increased from 0.65 (lighter)
```

## Mathematical Formula

```go
position = index / total  // 0.0 to 1.0

// Hue calculation (smooth gradient)
if position < 0.25:
    hue = 240 - (position / 0.25) * 40      // Blue → Cyan
else if position < 0.50:
    hue = 200 - ((position - 0.25) / 0.25) * 60  // Cyan → Green
else if position < 0.75:
    hue = 140 - ((position - 0.50) / 0.25) * 50  // Green → Yellow
else:
    hue = 90 - ((position - 0.75) / 0.25) * 60   // Yellow → Red

// Final color
color = oklch(0.65, 0.15, hue°)
```

## Testing Your Colors

### In Browser Console
```javascript
// Test color at position 0.5 (middle)
document.body.style.backgroundColor = 'oklch(0.65 0.15 140deg)';

// Test if browser supports OKLCH
CSS.supports('color', 'oklch(0.65 0.15 180deg)');  // true/false
```

### In PostgreSQL (Testing Data)
```sql
-- Get word distribution
SELECT
    CASE
        WHEN frequency > 200 THEN 'Hot (Red)'
        WHEN frequency > 100 THEN 'Warm (Yellow)'
        WHEN frequency > 50 THEN 'Cool (Green)'
        ELSE 'Cold (Blue)'
    END as color_range,
    COUNT(*) as word_count
FROM word_frequencies
GROUP BY color_range;
```

## Advanced: Extending the Palette

Want a 3-color gradient? Here's how:

```go
// Blue → Yellow → Red (current)
if position < 0.50:
    hue = 240 - (position / 0.50) * 150  // Blue to Yellow
else:
    hue = 90 - ((position - 0.50) / 0.50) * 60  // Yellow to Red
```

Want a 5-color gradient?

```go
// Blue → Cyan → Green → Yellow → Orange → Red
if position < 0.20:
    hue = 240 - (position / 0.20) * 40   // Blue → Cyan
else if position < 0.40:
    hue = 200 - ((position - 0.20) / 0.20) * 60  // Cyan → Green
else if position < 0.60:
    hue = 140 - ((position - 0.40) / 0.20) * 50  // Green → Yellow
else if position < 0.80:
    hue = 90 - ((position - 0.60) / 0.20) * 40   // Yellow → Orange
else:
    hue = 50 - ((position - 0.80) / 0.20) * 20   // Orange → Red
```

## References

- [OKLCH Color Space](https://bottosson.github.io/posts/oklab/)
- [CSS Color Module Level 4](https://www.w3.org/TR/css-color-4/#ok-lab)
- [MDN: oklch()](https://developer.mozilla.org/en-US/docs/Web/CSS/color_value/oklch)
- [Can I Use: oklch()](https://caniuse.com/mdn-css_types_color_oklch)

---

**Enjoy your perceptually perfect color gradient!** 🌈
