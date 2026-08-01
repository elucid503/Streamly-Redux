import type { Transition, Variants } from "framer-motion";

/** Soft spring for panels, sheets, and pills. */
export const softSpring: Transition = {

  type: "spring",
  stiffness: 420,
  damping: 34,
  mass: 0.85,

};

/** Slightly snappier for small menus and chips. */
export const snappySpring: Transition = {

  type: "spring",
  stiffness: 520,
  damping: 36,
  mass: 0.7,

};

export const fadeTransition: Transition = {

  duration: 0.18,
  ease: [0.22, 1, 0.36, 1],

};

export const backdropVariants: Variants = {

  hidden: { opacity: 0 },
  visible: { opacity: 1 },
  exit: { opacity: 0 },

};

export const popoverVariants: Variants = {

  hidden: { opacity: 0, scale: 0.96, y: -6 },
  visible: { opacity: 1, scale: 1, y: 0 },
  exit: { opacity: 0, scale: 0.96, y: -4 },

};

export const sheetVariants: Variants = {

  hidden: { opacity: 0, scale: 0.97, y: 8 },
  visible: { opacity: 1, scale: 1, y: 0 },
  exit: { opacity: 0, scale: 0.98, y: 6 },

};

export const noticeVariants: Variants = {

  hidden: { opacity: 0, y: 12, scale: 0.96 },
  visible: { opacity: 1, y: 0, scale: 1 },
  exit: { opacity: 0, y: 8, scale: 0.98 },

};

export const pageFade: Variants = {

  hidden: { opacity: 0, y: 6 },
  visible: { opacity: 1, y: 0 },
  exit: { opacity: 0, y: -4 },

};
