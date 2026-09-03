// Vite 5+ 移除了 globEager, 改用 glob(..., { eager: true })
const Mixins = import.meta.glob("./svg/*.svg", { eager: true })
export default {
    mixins: Object.values(Mixins).map((v) => v.default)
}
