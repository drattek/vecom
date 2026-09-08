import { SectionState } from "@/components/product-detail/ProductDetailPrimitives"
import {
    ProductAttachmentsCard,
    ProductImagesCard,
    ProductVideosCard,
} from "@/components/product-detail/ProductMediaEditor"
import { useProductSection } from "@/lib/product-detail"
import { productMediaSectionSchema } from "@/lib/schemas/product-details"
import { useParams } from "react-router-dom"

export function ProductMediaSection() {
    const { productId } = useParams<{ productId: string }>()
    const { data, isLoading, isError } = useProductSection(productId, "media", productMediaSectionSchema)

    if (isLoading) {
        return <SectionState state="loading" />
    }

    if (isError || !data) {
        return <SectionState state="error" message="No se pudo cargar la multimedia del producto." />
    }

    return (
        <div className="flex flex-col gap-4">
            <ProductImagesCard images={data.images} />
            <ProductVideosCard videos={data.videos} />
            <ProductAttachmentsCard attachments={data.attachments} />
        </div>
    )
}
