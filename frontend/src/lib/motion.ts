export const fadeSlideUp = {
  hidden:  { opacity: 0, y: 16 },
  visible: {
    opacity: 1, y: 0,
    transition: { duration: 0.22, ease: [0.16, 1, 0.3, 1] }
  },
}

export const staggerContainer = {
  hidden:  {},
  visible: { transition: { staggerChildren: 0.06, delayChildren: 0.08 } },
}

export const scaleIn = {
  hidden:  { opacity: 0, scale: 0.88 },
  visible: {
    opacity: 1, scale: 1,
    transition: { type: 'spring', stiffness: 400, damping: 28 }
  },
}

export const hoverLift = {
  whileHover: { y: -3, boxShadow: '0 8px 24px rgba(99,102,241,.15)' },
  whileTap:   { y: 0, scale: 0.99 },
  transition: { type: 'spring', stiffness: 400, damping: 30 },
}
