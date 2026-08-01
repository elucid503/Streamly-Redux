import * as React from "react"
import * as SliderPrimitive from "@radix-ui/react-slider"

import { cn } from "@/lib/cn"

function Slider({
  className,
  defaultValue,
  value,
  min = 0,
  max = 100,
  ...props
}: React.ComponentProps<typeof SliderPrimitive.Root>) {
  const _values = React.useMemo(
    () =>
      Array.isArray(value)
        ? value
        : Array.isArray(defaultValue)
          ? defaultValue
          : [min, max],
    [value, defaultValue, min, max]
  )

  return (
    <SliderPrimitive.Root
      data-slot="slider"
      defaultValue={defaultValue}
      value={value}
      min={min}
      max={max}
      className={cn(
        "relative flex h-4 w-full min-w-0 touch-none items-center select-none data-[disabled]:opacity-50",
        className
      )}
      {...props}
    >
      <SliderPrimitive.Track
        data-slot="slider-track"
        className="relative h-[3px] w-full grow overflow-hidden rounded-full bg-white/20"
      >
        <SliderPrimitive.Range
          data-slot="slider-range"
          className="absolute h-full rounded-full bg-white"
        />
      </SliderPrimitive.Track>
      {Array.from({ length: _values.length }, (_, index) => (
        <SliderPrimitive.Thumb
          data-slot="slider-thumb"
          key={index}
          className={cn(
            "block size-3.5 shrink-0 rounded-full bg-white",
            // Solid fill + soft shadow only — borders/rings alias badly on small circles.
            "shadow-[0_1px_2px_rgba(0,0,0,0.55),0_0_0_1px_rgba(0,0,0,0.25)]",
            "transition-transform duration-150 ease-out will-change-transform",
            "hover:scale-110 focus-visible:scale-110 focus-visible:outline-hidden",
            "disabled:pointer-events-none",
          )}
        />
      ))}
    </SliderPrimitive.Root>
  )
}

export { Slider }
