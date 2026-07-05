// Grade badge colors, aligned with the SSL Labs ordering. Returns Tailwind
// class strings for light and dark themes.
export function gradeClasses(grade?: string): string {
  const g = (grade ?? "").toUpperCase();
  if (g === "A+" || g === "A" || g === "A-")
    return "bg-green-100 text-green-700 dark:bg-green-950 dark:text-green-300";
  if (g === "B")
    return "bg-lime-100 text-lime-700 dark:bg-lime-950 dark:text-lime-300";
  if (g === "C")
    return "bg-yellow-100 text-yellow-700 dark:bg-yellow-950 dark:text-yellow-300";
  if (g === "D" || g === "E")
    return "bg-orange-100 text-orange-700 dark:bg-orange-950 dark:text-orange-300";
  if (g === "F" || g === "T" || g === "M")
    return "bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300";
  return "bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400";
}

export function gradeLabel(grade?: string): string {
  return grade && grade !== "" ? grade : "n/a";
}
