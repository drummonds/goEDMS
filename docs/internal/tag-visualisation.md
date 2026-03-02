# Tag Visualisation

How tags are rendered across godocs and how to reuse the patterns in other projects (e.g. godocs-inbox).

## Rendering patterns

Tags appear in five contexts, each with a different size/interaction level:

| Context | Template | Size | Interactive | Notes |
|---------|----------|------|-------------|-------|
| Document card | `partials/document_card.html` | `.tag-chip` (tiny) | No | Truncated to 7ch, sidebar column |
| Document view | `document.html` | `.tag.is-medium` | No | Read-only display |
| Document edit | `document_edit.html` | `.tag` (medium) | Delete button | Each tag has an inline remove form |
| Quick tags | `partials/quick_tags.html` | `.button.is-small` | Add on click | Greyed out with tick when assigned |
| Archive table | `archive.html` | `.tag.is-small` | No | Inline in table cell |

### Common markup

Every tag uses inline `style` for its colour rather than a CSS class, because colours are user-defined per tag:

```html
<span class="tag" style="background-color: {{ tag.Color }}; color: white;">
    {{ tag.Name }}
</span>
```

Variants differ only in Bulma size modifier (`is-small`, `is-medium`) and whether a delete/add control is included.

### Card chip (partials/document_card.html)

Compact tag shown in the card sidebar. Truncated with ellipsis to avoid overflow:

```html
<span class="tag tag-chip"
      style="background-color: {{ tag.Color }}; color: white;"
      title="{{ tag.Name }}">{{ tag.Name }}</span>
```

CSS (defined in `base.html` and `webapp.css`):

```css
.tag-chip {
    font-size: 0.7rem;
    max-width: 7ch;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
```

### Editable tag (document_edit.html)

Tag with an inline form for removal:

```html
<span class="tag" style="background-color: {{ tag.Color }}; color: white;">
    {{ tag.Name }}
    <form method="POST" action="/document/{{ doc.ULID }}/tags/{{ tag.ID }}/remove"
          style="display:inline; margin:0; padding:0;">
        <button type="submit" class="delete is-small"></button>
    </form>
</span>
```

### Quick tag button (partials/quick_tags.html)

Top-15 most-used tags shown as clickable buttons. Already-assigned tags are greyed out:

```html
<!-- Assigned -->
<span class="button is-small is-static" style="opacity: 0.5; ...">
    &#10003; {{ qt.Name }}
</span>

<!-- Available — click to add -->
<form method="POST" action="{{ form_action_prefix }}/tags">
    <input type="hidden" name="tag_id" value="{{ qt.ID }}">
    <button type="submit" class="button is-small"
            style="background-color: {{ qt.Color }}; color: white;">
        {{ qt.Name }}
    </button>
</form>
```

## Composability with godocs-inbox

godocs-inbox needs to display and assign tags during the upload/triage workflow. The goal is to share tag rendering logic rather than duplicate it.

### What can be reused

1. **Tag data model** -- godocs-inbox fetches tags via `GET /api/tags` and gets the same `{id, name, color, tag_group}` objects.

2. **Display-only chip** -- the card chip and read-only tag markup are pure HTML+CSS with no godocs-specific dependencies. Any pongo2 template can render them given a tag object with `Name` and `Color`.

3. **Quick tags partial** -- `partials/quick_tags.html` is parameterised via `form_action_prefix` so it already works for different form targets. godocs-inbox can include it directly if it uses pongo2 templates, or replicate the pattern with its own form action.

### What needs adaptation

1. **Remove/add forms** -- form actions (`/document/:ulid/tags/:id/remove`) are godocs-specific routes. godocs-inbox needs its own endpoints or should call the godocs API directly.

2. **Tag group enforcement** -- the one-tag-per-group constraint is enforced by a DB trigger in godocs. External tools adding grouped tags via the API get the same enforcement. Tools that pre-filter available tags should call `GET /api/tags/groups` to understand the group structure.

### Recommended approach

Keep tag rendering as **partials with minimal assumptions**:

- `partials/tag_chip.html` -- display-only, needs `tag.Name` and `tag.Color`
- `partials/tag_editable.html` -- needs `tag`, `doc.ULID`, and a `remove_action` URL
- `partials/quick_tags.html` -- already parameterised via `form_action_prefix`

External projects (godocs-inbox) can either:
- **Include the partials** if they embed godocs templates
- **Copy the HTML pattern** -- it's 3-5 lines of Bulma markup per variant
- **Use the API** -- fetch tags, render client-side using the same colour/name fields

## Tag group visualisation

Tags in groups are visually indistinguishable from free tags on document cards (both use the same chip). The group structure is visible in:

- **Tag manager** (`/tags`) -- tags are grouped by `tag_group` in the table
- **Document edit** -- grouped tags appear alongside free tags; the DB trigger prevents assigning two tags from the same group
- **`.tags.json` sidecar** -- separates `tags` (free) from `tag_groups` (grouped)

For richer group visualisation (e.g. showing group headers on cards), a future partial could render grouped tags separately.
