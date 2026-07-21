import {
  GetWallpaper,
  GetWallpaperGlass,
  SetWallpaper,
  SetWallpaperGlass,
  PickAndUploadWallpaper,
  ListCustomWallpapers,
  GetCustomWallpaperImage,
  DeleteCustomWallpaper,
} from '@wails/go/main/App';
import type { dto } from '@wails/go/models';

export const DEFAULT_WALLPAPER_GLASS = 20;

export type WallpaperUploadResult = dto.WallpaperUploadResult;
export type WallpaperImageResult = dto.WallpaperImageResult;

export const wallpaperService = {
  get(): Promise<string> {
    try {
      return GetWallpaper().catch(() => 'default');
    } catch {
      return Promise.resolve('default');
    }
  },
  set(id: string): Promise<dto.OperationResult> {
    return SetWallpaper(id);
  },
  getGlass(): Promise<number> {
    try {
      return GetWallpaperGlass().catch(() => DEFAULT_WALLPAPER_GLASS);
    } catch {
      return Promise.resolve(DEFAULT_WALLPAPER_GLASS);
    }
  },
  setGlass(opacity: number): Promise<dto.OperationResult> {
    return SetWallpaperGlass(opacity);
  },
  // pickAndUpload opens the native image picker; the chosen image is imported
  // as a custom wallpaper and selected. result.canceled is true when the user
  // dismissed the dialog.
  pickAndUpload(): Promise<WallpaperUploadResult> {
    // The binding is denylisted in web mode (native dialog); surface that as a
    // normal error result instead of an unhandled rejection.
    return PickAndUploadWallpaper().catch((error: unknown) => ({
      success: false,
      error: error instanceof Error ? error.message : String(error),
    } as WallpaperUploadResult));
  },
  listCustom(): Promise<WallpaperUploadResult> {
    return ListCustomWallpapers();
  },
  // getImage resolves a "custom:<file>" id to a base64 data URI for rendering
  // (uploads are not bundled assets, so the frontend can't use a /path URL).
  getImage(id: string): Promise<WallpaperImageResult> {
    return GetCustomWallpaperImage(id);
  },
  deleteCustom(id: string): Promise<WallpaperUploadResult> {
    return DeleteCustomWallpaper(id);
  },
};
