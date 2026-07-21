import { ICON_SIZE_MAX, ICON_SIZE_MIN, ICON_SIZE_STEP, ORDER_LABELS, type OrderMode } from '@modules/home/home-modules';

interface AppGridToolbarProps {
  search: string;
  onSearchChange: (value: string) => void;
  searchPlaceholder?: string;
  orderMode: OrderMode;
  onOrderChange: (mode: OrderMode) => void;
  iconSize: number;
  onIconSizeChange: (size: number) => void;
  /** Aria/title label for the size control. Defaults to 'Card size'. */
  sizeLabel?: string;
}

// AppGridToolbar renders the three Apps-screen controls (search, order-by,
// card-size) that are shared between the Home/Apps screen and any settings
// page that uses the same card-grid layout (e.g. Settings › Theme).
// All state is owned by the parent; this component is purely presentational.
export function AppGridToolbar({
  search,
  onSearchChange,
  searchPlaceholder = 'Search...',
  orderMode,
  onOrderChange,
  iconSize,
  onIconSizeChange,
  sizeLabel = 'Card size',
}: AppGridToolbarProps) {
  return (
    <div className="home-toolbar">
      <div className="home-search">
        <span className="nav-icon home-search-icon material-symbols-outlined" aria-hidden="true">search</span>
        <input
          type="text"
          className="home-search-input"
          placeholder={searchPlaceholder}
          value={search}
          onChange={(event) => onSearchChange(event.target.value)}
          aria-label={searchPlaceholder}
        />
        {search && (
          <button
            type="button"
            className="chat-search-clear"
            onClick={() => onSearchChange('')}
            aria-label="Clear search"
          >
            <span className="material-symbols-outlined" aria-hidden="true">close</span>
          </button>
        )}
      </div>

      <select
        className="home-order-select"
        value={orderMode}
        onChange={(event) => onOrderChange(event.target.value as OrderMode)}
        aria-label="Order by"
        title="Order by"
      >
        {(Object.keys(ORDER_LABELS) as OrderMode[]).map((mode) => (
          <option key={mode} value={mode}>Order by: {ORDER_LABELS[mode]}</option>
        ))}
      </select>

      <div className="home-iconsize" title={sizeLabel}>
        <button
          type="button"
          className="home-iconsize-btn"
          onClick={() => onIconSizeChange(ICON_SIZE_MIN)}
          title="Set minimum card size"
          aria-label="Set minimum card size"
        >
          <span className="material-symbols-outlined" aria-hidden="true">apps</span>
        </button>
        <input
          type="range"
          className="home-size-range"
          min={ICON_SIZE_MIN}
          max={ICON_SIZE_MAX}
          step={ICON_SIZE_STEP}
          value={iconSize}
          onChange={(event) => onIconSizeChange(Number(event.target.value))}
          aria-label={sizeLabel}
        />
        <button
          type="button"
          className="home-iconsize-btn"
          onClick={() => onIconSizeChange(ICON_SIZE_MAX)}
          title="Set maximum card size"
          aria-label="Set maximum card size"
        >
          <span className="material-symbols-outlined" aria-hidden="true">grid_view</span>
        </button>
      </div>
    </div>
  );
}
