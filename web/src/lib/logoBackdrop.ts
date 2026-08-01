import { FastAverageColor } from "fast-average-color";

const picker = new FastAverageColor();
const cache = new Map<string, Promise<string>>();

export function logoBackdrop(image: HTMLImageElement): Promise<string> {

  const src = image.currentSrc || image.src;

  const cached = cache.get(src);

  if (cached) {

    return cached;

  }

  const pending = picker.getColorAsync(image, {

    algorithm: "dominant",
    ignoredColor: [0, 0, 0, 0, 40],
    silent: true,

  }).then((color) => contrastTint(color.value, color.isDark)).catch(() => "#f4f4f5");

  cache.set(src, pending);

  return pending;

}

function contrastTint([red, green, blue]: number[], darkLogo: boolean): string {

  const target = darkLogo ? 255 : 0;
  const amount = darkLogo ? 0.9 : 0.84;

  return `rgb(${mix(red, target, amount)} ${mix(green, target, amount)} ${mix(blue, target, amount)})`;

}

function mix(value: number, target: number, amount: number): number {

  return Math.round(value + (target - value) * amount);

}
