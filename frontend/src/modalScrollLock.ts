const modalScrollOwners = new Set<string>();

function syncModalScrollLock(): void {
  const locked = modalScrollOwners.size > 0;
  document.documentElement.classList.toggle("modal-scroll-locked", locked);
  document.body.classList.toggle("modal-scroll-locked", locked);
}

export function lockModalScroll(owner: string): void {
  modalScrollOwners.add(owner);
  syncModalScrollLock();
}

export function unlockModalScroll(owner: string): void {
  modalScrollOwners.delete(owner);
  syncModalScrollLock();
}
