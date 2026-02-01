// Parse room ID from URL query parameter
export function getRoomIdFromURL(): string | null {
  const params = new URLSearchParams(window.location.search);
  return params.get('room');
}

// Update URL with room ID
export function setRoomIdInURL(roomId: string): void {
  const url = new URL(window.location.href);
  url.searchParams.set('room', roomId);
  window.history.pushState({}, '', url.toString());
}

// Clear room ID from URL
export function clearRoomIdFromURL(): void {
  const url = new URL(window.location.href);
  url.searchParams.delete('room');
  window.history.pushState({}, '', url.toString());
}

// Generate shareable link
export function generateShareableLink(roomId: string): string {
  const url = new URL(window.location.href);
  url.searchParams.set('room', roomId);
  return url.toString();
}

// Copy shareable link to clipboard
export async function copyShareableLink(roomId: string): Promise<boolean> {
  try {
    const link = generateShareableLink(roomId);
    await navigator.clipboard.writeText(link);
    return true;
  } catch (error) {
    console.error('Failed to copy link:', error);
    return false;
  }
}
