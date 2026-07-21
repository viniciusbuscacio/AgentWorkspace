// Skills sub-navigation memory. Leaving the Skills module (e.g. opening a
// chat) unmounts it and loses in-component state; coming back used to dump the
// user on the list. Persist the last open skill for the session so returning
// lands on Skills › <skill> again.
//
// sessionStorage (not localStorage) on purpose: this is a within-session
// convenience, not a cross-restart preference. A fresh app launch starts clean.

const SKILL_KEY = 'skills-last-skill';

export function readLastSkillId(): string | null {
  try {
    return sessionStorage.getItem(SKILL_KEY) || null;
  } catch {
    return null;
  }
}

export function writeLastSkillId(id: string | null): void {
  try {
    if (id) sessionStorage.setItem(SKILL_KEY, id);
    else sessionStorage.removeItem(SKILL_KEY);
  } catch {
    /* ignore */
  }
}
