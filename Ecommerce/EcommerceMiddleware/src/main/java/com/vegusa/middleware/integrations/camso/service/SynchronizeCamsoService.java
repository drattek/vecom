package com.vegusa.middleware.integrations.camso.service;

import com.vegusa.middleware.integrations.camso.dto.AttributesCamsoDTO;
import com.vegusa.middleware.integrations.camso.entity.ImageCamso;
import com.vegusa.middleware.integrations.camso.entity.ProductCamso;
import com.vegusa.middleware.integrations.camso.repository.ImageCamsoRepository;
import com.vegusa.middleware.integrations.camso.repository.ProductCamsoRepository;
import com.vegusa.middleware.integrations.jumpseller.client.image.ImageJumpsellerClient;
import com.vegusa.middleware.integrations.jumpseller.client.product.ProductJumpsellerClient;
import com.vegusa.middleware.integrations.jumpseller.dto.*;
import com.vegusa.middleware.integrations.jumpseller.entity.SyncJumpsellerProduct;
import com.vegusa.middleware.integrations.jumpseller.repository.SyncProductJumpsellerRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpMethod;
import org.springframework.http.HttpStatusCode;
import org.springframework.http.client.ClientHttpResponse;
import org.springframework.http.client.SimpleClientHttpRequestFactory;
import org.springframework.stereotype.Service;
import org.springframework.web.client.RequestCallback;
import org.springframework.web.client.ResponseExtractor;
import org.springframework.web.client.RestTemplate;
import java.math.BigDecimal;
import java.math.RoundingMode;
import java.util.ArrayList;
import java.util.List;
import java.util.Objects;

@Service
public class SynchronizeCamsoService {
    private final RestTemplate restTemplate;

    @Autowired
    private ProductJumpsellerClient productClient;

    @Autowired
    private ImageJumpsellerClient imageClient;

    @Autowired
    private ProductCamsoRepository camsoRepository;

    @Autowired
    private ImageCamsoRepository imageRepository;

    @Autowired
    private SyncProductJumpsellerRepository jumpsellerRepository;

    public SynchronizeCamsoService(){
        SimpleClientHttpRequestFactory requestFactory = new SimpleClientHttpRequestFactory();
        requestFactory.setConnectTimeout(3000); // 3 segundos
        requestFactory.setReadTimeout(3000);
        this.restTemplate = new RestTemplate(requestFactory);
    }

    public void UpdateCamsoProducts(List<AttributesCamsoDTO> data){
        for (AttributesCamsoDTO item : data){
            ProductCamso product = camsoRepository.findByPartNumber(item.getPartNumber())
                    .orElseGet(() -> null);

            if (product != null){
                product.setDiameter(item.getDiameter());
                product.setWidth(item.getWidth());
                product.setHeight(item.getHeight());
                product.setLength(item.getLength());
                product.setWeight(item.getWeight());
                if (item.getBarcode() != null){
                    product.setBarcode(item.getBarcode());
                }

                camsoRepository.save(product);
                System.out.println("Updated item: " + item.getPartNumber());
            }
        }
        System.out.println("Updated ended");
    }

    public void UpdateCamsoImages(){
        List<ProductCamso> items = camsoRepository.findAll();

        for (ProductCamso item : items){
            String url = ValidateImage(item.getPartNumber());

            if (!url.isEmpty()){
                System.out.println("Image found: " + item.getPartNumber() + " - " + url);
                ImageCamso image = imageRepository.findByPartNumber(item.getPartNumber())
                        .orElseGet(() -> {
                            ImageCamso tmp = new ImageCamso();
                            tmp.setProductId(item.getId());
                            tmp.setPartNumber(item.getPartNumber());
                            return tmp;
                        });

                image.setUrl(url);

                imageRepository.save(image);
            }
        }
        System.out.println("Update ended");
    }

    public void CreateSync(BigDecimal changeType){
        //List<ProductCamso> items = camsoRepository.findBySync(false);
        List<ProductCamso> items = camsoRepository.getSyncProducts();

        for (ProductCamso item : items){
            System.out.println("Try item: " + item.getPartNumber());
            ImageCamso url = imageRepository.findByPartNumber(item.getPartNumber())
                    .orElseGet(() -> null);
            if (url != null){
                String name = item.getName() + " - CAMSO";
                BigDecimal price = getPrice(item.getPrice(),item.getCurrency());
                BigDecimal cmUnit = new BigDecimal("2.54");

                Product product = new Product();
                product.setName(name);
                product.setSku(item.getPartNumber());
                product.setStock(item.getStock().intValue());
                product.setBrand("CAMSO");
                product.setBarcode(item.getBarcode());
                product.setStatus("available");
                BigDecimal finalPrice;
                if (Objects.equals(item.getCurrency(), "USD")){
                    finalPrice = price.multiply(changeType).divide(new BigDecimal("0.65"), RoundingMode.HALF_UP);
                } else {
                    finalPrice = price.divide(new BigDecimal("065"), RoundingMode.HALF_UP);
                }
                product.setPrice(finalPrice.setScale(2, RoundingMode.HALF_UP));

                BigDecimal diameter = item.getDiameter().multiply(cmUnit);
                product.setDiameter(diameter.setScale(2, RoundingMode.HALF_UP));

                BigDecimal width = item.getWidth().multiply(cmUnit);
                product.setWidth(width.setScale(2, RoundingMode.HALF_UP));

                BigDecimal height = item.getHeight().multiply(cmUnit);
                product.setHeight(height.setScale(2, RoundingMode.HALF_UP));

                BigDecimal length = item.getLength().multiply(cmUnit);
                product.setLength(length.setScale(2, RoundingMode.HALF_UP));

                BigDecimal weight = item.getWeight().multiply(new BigDecimal("0.453592"));
                product.setWeight(weight.setScale(2, RoundingMode.HALF_UP));

                List<CategoryDTO> categories = new ArrayList<>();
                CategoryDTO rootCategory = new CategoryDTO();
                rootCategory.setId(2185804L);
                categories.add(rootCategory);

                CategoryDTO baseCategory = new CategoryDTO();
                baseCategory.setId(2222264L);
                categories.add(baseCategory);

                CategoryDTO subCategory = new CategoryDTO();
                subCategory.setId(2355888L);
                categories.add(subCategory);

                product.setCategories(categories.toArray(CategoryDTO[]::new));

                String description = getDescription(item.getName(), item.getPartNumber(), item.getBrand(), item.getCategory());
                product.setDescription(description);
                product.setMeta_description(item.getName() + " - " + item.getPartNumber());

                JumpsellerProductDto dto = new JumpsellerProductDto(product);

                try {
                    JumpsellerProductDto result = productClient.createProduct(dto).block();
                    if (result != null){
                        System.out.println("Created item: " + item.getPartNumber());
                        Product resultProduct = result.getProduct();
                        SyncJumpsellerProduct syncItem = new SyncJumpsellerProduct();
                        syncItem.setResponseId(resultProduct.getId());
                        syncItem.setInternalCode(resultProduct.getSku());
                        syncItem.setName(resultProduct.getName());
                        syncItem.setPageTitle(resultProduct.getPage_title());
                        syncItem.setDescription(resultProduct.getDescription());
                        syncItem.setMetaDescription(resultProduct.getMeta_description());
                        syncItem.setType(resultProduct.getType());
                        syncItem.setDaysToExpire(resultProduct.getDays_to_expire());
                        syncItem.setPrice(resultProduct.getPrice());
                        syncItem.setDiscount(resultProduct.getDiscount());
                        syncItem.setWeight(resultProduct.getWeight());
                        syncItem.setStock(resultProduct.getStock());
                        syncItem.setStockUnlimited(resultProduct.isStock_unlimited());
                        syncItem.setStockThreshold(resultProduct.getStock_threshold());
                        syncItem.setStockNotification(resultProduct.isStock_notification());
                        syncItem.setCostPerItem(resultProduct.getCost_per_item());
                        syncItem.setCompareAtPrice(resultProduct.getCompare_at_price());
                        syncItem.setSku(resultProduct.getSku());
                        syncItem.setBrand(resultProduct.getBrand());
                        syncItem.setBarcode(resultProduct.getBarcode());
                        syncItem.setFeatured(resultProduct.isFeatured());
                        syncItem.setShippingRequired(resultProduct.isShipping_required());
                        syncItem.setReviewsEnabled(resultProduct.isReviews_enabled());
                        syncItem.setStatus(resultProduct.getStatus());
                        syncItem.setCreatedAt(resultProduct.getCreated_at());
                        syncItem.setUpdatedAt(resultProduct.getUpdated_at());
                        syncItem.setPackageFormat(resultProduct.getPackage_format());
                        syncItem.setLength(resultProduct.getLength());
                        syncItem.setWidth(resultProduct.getWidth());
                        syncItem.setHeight(resultProduct.getHeight());
                        syncItem.setDiameter(resultProduct.getDiameter());
                        syncItem.setPermalink(resultProduct.getPermalink());
                        syncItem.setDataAreaId("CAMSO");

                        jumpsellerRepository.save(syncItem);

                        try {
                            Image img = new Image();
                            img.setUrl(url.getUrl());
                            img.setPosition(0L);
                            JumpsellerImageDTO imageDTO = new JumpsellerImageDTO(img);
                            JumpsellerImageDTO resultImage = imageClient.uploadImage(resultProduct.getId().toString(), imageDTO).block();

                            if (resultImage != null){
                                System.out.println("Upload image item: " + item.getPartNumber());
                                url.setSync(true);
                                imageRepository.save(url);
                            }
                        } catch (Exception e){
                            System.err.println("Error uploading image");
                        }

                        item.setSync(true);
                        camsoRepository.save(item);
                    }
                } catch (Exception e){
                    System.err.println("Error creating product");
                }
            }
        }

        System.out.println("Create process completed");
    }

    public void UpdateSync(BigDecimal changeType){
        SyncJumpsellerProduct[] syncItems = jumpsellerRepository.getSyncProducts("CAMSO");

        for (SyncJumpsellerProduct syncItem : syncItems) {
            ProductCamso item = camsoRepository.findByPartNumber(syncItem.getInternalCode())
                    .orElseGet(() -> null);
            if (item != null){
                BigDecimal price = getPrice(item.getPrice(), item.getCurrency());
                String description = getDescription(item.getName(), item.getPartNumber(), item.getBrand(), item.getCategory());

                Product product = new Product();
                product.setName(syncItem.getName());
                BigDecimal finalPrice;
                if (Objects.equals(item.getCurrency(), "USD")){
                    finalPrice = price.multiply(changeType).divide(new BigDecimal("0.65"), RoundingMode.HALF_UP);
                } else {
                    finalPrice = price.divide(new BigDecimal("065"), RoundingMode.HALF_UP);
                }
                product.setPrice(finalPrice.setScale(2, RoundingMode.HALF_UP));
                product.setStock(item.getStock().intValue());
                product.setDescription(description);
                product.setMeta_description(syncItem.getName() + " - " + item.getPartNumber());

                JumpsellerProductDto dto = new JumpsellerProductDto(product);

                if (item.getStock().intValue() != syncItem.getStock()){
                    JumpsellerProductDto result = productClient.updateProduct(syncItem.getResponseId(), dto).block();
                    if (result != null){
                        syncItem.setPrice(result.getProduct().getPrice());
                        syncItem.setDescription(result.getProduct().getDescription());
                        syncItem.setStock(result.getProduct().getStock());
                        jumpsellerRepository.save(syncItem);
                        System.out.println("Updated item: " + item.getPartNumber());
                    }
                }
            }
        }

        System.out.println("Update completed");
    }

    private BigDecimal getPrice(String price, String currency){
        String tmp = price.replace(currency, "").trim();
        return new BigDecimal(tmp);
    }

    private String ValidateImage(String code){
        String imageUrl = "https://vconstorage2.blob.core.windows.net/veg-ecomm-products/camso_" + code + ".webp";
        try {
            RequestCallback requestCallback = request -> request.getHeaders().set("User-Agent", "Mozilla/5.0");

            ResponseExtractor<HttpStatusCode> responseExtractor = ClientHttpResponse::getStatusCode;

            HttpStatusCode statusCode = restTemplate.execute(imageUrl, HttpMethod.HEAD, requestCallback, responseExtractor);

            if (statusCode != null && statusCode.is2xxSuccessful()) {
                return imageUrl;
            } else {
                return "";
            }

        } catch (Exception e){
            return "";
        }
    }

    private String getDescription (String name, String partNumber, String brand, String category){
        return """
            <h3 class="item-description-title">%s – %s | Camso</h3>
            <p><strong>Número de parte:</strong> %s<br>
            <strong>Categoría:</strong> %s<br>
            <strong>Modelo:</strong> %s<br>
            <strong>Distribuidor autorizado:</strong> Refacciones Vegusa (Grupo Vegusa)</p>

            <h3 class="item-description-subtitle">Descripción general:</h3>
            <p>La llanta <strong>%s</strong> de la línea <strong>%s</strong> de Camso está diseñada para ofrecer un rendimiento superior en aplicaciones de <strong>%s</strong>. Fabricada con compuestos de alta calidad y tecnología avanzada, esta llanta garantiza durabilidad, tracción y resistencia en los entornos más exigentes.</p>

            <h3 class="item-description-subtitle">Características destacadas:</h3>
            <ul>
                <li>✅ <strong>Alta resistencia al desgaste</strong> para una vida útil prolongada.</li>
                <li>✅ <strong>Excelente tracción</strong> en superficies difíciles.</li>
                <li>✅ <strong>Diseño optimizado</strong> para mejorar la estabilidad y el comfort.</li>
                <li>✅ <strong>Ideal para aplicaciones de %s</strong>, como montacargas, maquinaria pesada, etc.</li>
            </ul>

            <h3 class="item-description-subtitle">Aplicaciones comunes:</h3>
            <ul>
                <li>Vehículos de construcción como retroexcavadoras, cargadores frontales, etc.</li>
                <li>Equipos de manejo de materiales como montacargas, tractores industriales, etc.</li>
            </ul>

            <h3 class="item-description-subtitle">¿Por qué elegir Camso?</h3>
            <p>Camso, parte de Michelin, es líder mundial en soluciones de movilidad fuera de carretera. Sus productos están diseñados para maximizar el rendimiento y reducir los costos operativos.</p>
            """.formatted(name, brand, partNumber, category, brand, name, brand, category, category);
    }
}
