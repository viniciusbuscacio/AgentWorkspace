# Sidebar compact states

The app has more than one compact sidebar state. Do not assume that a closed/collapsed sidebar only means `sidebar-mode-icon`.

The React state already exposes the logical compact condition:

```ts
const collapsed = sidebar.mode === 'icon' || sidebar.mode === 'peek';
```

For UI elements that should not exist while the sidebar is compact, prefer conditional rendering based on `collapsed` instead of hiding only one visual mode with CSS.

Example for the profile caret:

```tsx
{!collapsed && (
  <span className="profile-caret material-symbols-outlined" aria-hidden="true">
    expand_less
  </span>
)}
```

Avoid fragile CSS like:

```css
#app.sidebar-mode-icon .profile-caret {
  display: none;
}
```

That only covers `icon`, but not `peek` (`sidebar-mode-peek`).
