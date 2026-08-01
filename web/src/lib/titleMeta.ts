/** Player subtitle for movies — year first, then rating / primary genre when present. */
export function movieCaption(meta: {
  year?: number;
  rating?: string;
  genres?: string[];
}): string | undefined {

  const parts: string[] = [];

  if (meta.year && meta.year > 0) {

    parts.push(String(meta.year));

  }

  if (meta.rating) {

    parts.push(meta.rating);

  }

  if (meta.genres?.[0]) {

    parts.push(meta.genres[0]);

  }

  if (parts.length === 0) {

    return undefined;

  }

  return parts.join(" · ");

}
