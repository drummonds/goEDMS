# Tag Editor Spec

Reusable tag editor partial for adding and removing tags on documents. Companion to `tag-visualisation.md` (display-only patterns) and `tagging.md` (data model).

## Partial

`templates/partials/tag_editor.html`

Renders three sections:

1. **Current Tags** — coloured chips with inline remove forms
2. **Quick Tags** — top-15 most-used tags as one-click buttons (delegates to `partials/quick_tags.html`)
3. **Add Tag** — dropdown of remaining tags with submit button

## Context variables

| Variable | Required | Type | Description |
|----------|----------|------|-------------|
| `tags` | yes | `[]Tag` | Currently assigned tags (`.ID`, `.Name`, `.Color`) |
| `available_tags` | yes | `[]Tag` | Tags not yet assigned |
| `remove_action_prefix` | yes | string | URL prefix for remove forms — `/{id}/remove` is appended |
| `add_action` | yes | string | Form action URL for adding a tag |
| `quick_tags` | no | `[]QuickTag` | Passed through to `quick_tags.html` |
| `form_action_prefix` | no | string | Passed through to `quick_tags.html` |
| `add_button_text` | no | string | Submit button label (default: "Add") |
| `no_tags_message` | no | string | Empty-state message (default: "No tags assigned.") |
| `extra_hidden_name` | no | string | Name of an extra hidden field on all forms |
| `extra_hidden_value` | no | string | Value of the extra hidden field |

## Usage: single document edit

```html
{% with remove_action_prefix="/document/"+doc.ULID.String+"/tags"
       add_action="/document/"+doc.ULID.String+"/tags" %}
{% include "partials/tag_editor.html" %}
{% endwith %}
```

`quick_tags` and `form_action_prefix` are set in the page handler's pongo2 context.

## Usage: bulk edit

```html
{% with tags=common_tags
       remove_action_prefix="/documents/bulk-edit/tags"
       add_action="/documents/bulk-edit/tags"
       add_button_text="Add to All"
       no_tags_message="No tags common to all selected documents."
       extra_hidden_name="ulids"
       extra_hidden_value=ulids %}
{% include "partials/tag_editor.html" %}
{% endwith %}
```

The `extra_hidden_name`/`extra_hidden_value` pair injects a hidden `<input>` into both remove and add forms, passing the bulk selection through each POST.

## Usage: godocs-inbox (external)

godocs-inbox can reuse the partial in two ways:

### Option A: embed the partial

If godocs-inbox uses pongo2 templates and embeds (or symlinks) the godocs partials directory, it can include the partial directly with its own action URLs:

```html
{% with tags=inbox_tags
       remove_action_prefix="/inbox/"+item.ID+"/tags"
       add_action="/inbox/"+item.ID+"/tags" %}
{% include "partials/tag_editor.html" %}
{% endwith %}
```

### Option B: replicate the pattern

The tag editor is ~40 lines of Bulma HTML. Copy the three sections (current tags, quick tags, add dropdown) and wire up to local form actions. The markup is intentionally simple to make this viable.

### Either way, the API contract is the same

- Remove: `POST {remove_action_prefix}/{tag_id}/remove`
- Add: `POST {add_action}` with `tag_id` form field
- Tags fetched via `GET /api/tags` return the same `{id, name, color, tag_group}` shape

## Related partials

| Partial | Purpose |
|---------|---------|
| `partials/tag_editor.html` | Full tag editor (this spec) |
| `partials/quick_tags.html` | One-click buttons for frequent tags |
| `partials/document_card.html` | Display-only tag chips on cards |

## Design decisions

**Why `{% with %}` instead of separate context keys?** The `with` block scopes variables to the include, avoiding collision with the parent template's variables (e.g. `tags` in document_edit means "assigned tags" but the page also has `available_tags`). For bulk edit, `tags=common_tags` remaps the variable name cleanly.

**Why `extra_hidden_name`/`extra_hidden_value` instead of wrapping in a form?** Each tag removal is its own inline form (no JavaScript). The hidden field must appear inside each form. A wrapper form would require JavaScript to handle multiple submit buttons.

**Why not extract tag removal into its own partial?** The removal markup is 5 lines including the hidden field logic. A sub-partial would add indirection without meaningful reuse beyond what `tag_editor.html` already provides.
