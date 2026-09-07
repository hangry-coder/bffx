import React from 'react';
import { X } from 'lucide-react';
import { motion, AnimatePresence } from 'motion/react';

export const Modal = ({
  isOpen,
  onClose,
  title,
  children,
}: {
  isOpen: boolean;
  onClose: () => void;
  title: string;
  children: React.ReactNode;
}) => (
  <AnimatePresence>
    {isOpen && (
      <div className="fixed inset-0 z-[100] flex items-center justify-center p-4">
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          onClick={onClose}
          className="absolute inset-0 bg-background/80 backdrop-blur-sm"
        />
        <motion.div
          initial={{ opacity: 0, scale: 0.95, y: 20 }}
          animate={{ opacity: 1, scale: 1, y: 0 }}
          exit={{ opacity: 0, scale: 0.95, y: 20 }}
          className="relative w-full max-w-lg bg-surface-card border border-border-subtle rounded-xl shadow-2xl overflow-hidden"
        >
          <div className="p-6 border-b border-border-subtle flex justify-between items-center bg-surface-dim">
            <h3 className="font-display text-lg font-semibold text-text-main">{title}</h3>
            <button onClick={onClose} className="text-[#8c90a1] hover:text-text-main transition-colors">
              <X size={20} />
            </button>
          </div>
          <div className="p-6 space-y-4">{children}</div>
        </motion.div>
      </div>
    )}
  </AnimatePresence>
);
