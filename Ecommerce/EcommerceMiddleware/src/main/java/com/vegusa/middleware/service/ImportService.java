package com.vegusa.middleware.service;

import com.vegusa.middleware.constants.AttributeParameters;
import com.vegusa.middleware.constants.DataArea;
import com.vegusa.middleware.constants.ProductInterface;
import com.vegusa.middleware.dto.ImporMeasurementDTO;
import com.vegusa.middleware.dto.ImportImageDTO;
import com.vegusa.middleware.dto.ImportSeoDTO;
import com.vegusa.middleware.entity.*;
import com.vegusa.middleware.repository.*;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Mono;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.time.Instant;
import java.util.*;

@Service
public class ImportService {
    @Autowired
    private ProductAttributeValueRepository valueRepository;

    @Autowired
    private InterfaceItemsRepository productRepository;

    @Autowired
    private CompanyRepository companyRepository;

    @Autowired
    private InterfaceRepository interfaceRepository;

    @Autowired
    private ProductAttributeRepository productAttributeRepository;

    @Autowired
    private ProductAttributeValuesRepository productAttributeValueRepository;

    @Autowired
    private ProductImageRepository imageRepository;

    @Autowired
    private ProductsRepository productsRepository;

    @Autowired
    private ProductPendingsRepository pendingsRepository;

    public Mono<Void> importSeoData(List<ImportSeoDTO> dataList){
        Company company = companyRepository.getCompany(DataArea.MSB.name());
        Interface interfaceDYN = interfaceRepository.getInterface(ProductInterface.DYN.name(), DataArea.MSB.name());
        ProductAttribute seoTitleAttribute = productAttributeRepository.getProductAttribute(AttributeParameters.SEO_TITLE.name(), DataArea.MSB.name());
        ProductAttribute metaDescriptionAttribute = productAttributeRepository.getProductAttribute(AttributeParameters.META_DESCRIPTION.name(), DataArea.MSB.name());

        List<String> itemIds = new ArrayList<>();
        Map<String, ImportSeoDTO> seoData = new HashMap<>();
        for (ImportSeoDTO item : dataList){
            itemIds.add(item.getInternal());
            seoData.put(item.getInternal(), item);
        }

        List<InterfaceItems> products = productRepository.getProducts(itemIds, "MSB");

        for (InterfaceItems product : products){
            ProductAttributeValue title_value = valueRepository.getProductAttributeValue(
                    AttributeParameters.SEO_TITLE.name(),
                    product.getItemId(),
                    ProductInterface.DYN.name(),
                    DataArea.MSB.name()
            );

            if (title_value == null){
                System.out.println("Product " + product.getItemId() + " doesn't had seo title");
                ProductAttributeValue seoTitle = new ProductAttributeValue();
                seoTitle.setItemId(product.getItemId());
                seoTitle.setInterfaceField(interfaceDYN);
                seoTitle.setProductattribute(seoTitleAttribute);
                seoTitle.setValue(seoData.get(product.getItemId()).getTitle());
                seoTitle.setSkipNull("TRUE");
                seoTitle.setCompany(company);
                seoTitle.setProductRefRec(product);
                seoTitle.setCreatedAt(new Date());
                seoTitle.setUpdatedAt(new Date());

                valueRepository.save(seoTitle);
            }

            ProductAttributeValue meta_description = valueRepository.getProductAttributeValue(
                    AttributeParameters.META_DESCRIPTION.name(),
                    product.getItemId(),
                    ProductInterface.DYN.name(),
                    DataArea.MSB.name()
            );
            if (meta_description == null){
                System.out.println("Product " + product.getItemId() + " doesn't had meta description");
                ProductAttributeValue metaDescription = new ProductAttributeValue();
                metaDescription.setItemId(product.getItemId());
                metaDescription.setInterfaceField(interfaceDYN);
                metaDescription.setProductattribute(metaDescriptionAttribute);
                metaDescription.setValue(seoData.get(product.getItemId()).getDescription());
                metaDescription.setSkipNull("TRUE");
                metaDescription.setCompany(company);
                metaDescription.setProductRefRec(product);
                metaDescription.setCreatedAt(new Date());
                metaDescription.setUpdatedAt(new Date());

                valueRepository.save(metaDescription);
            }
        }

        return Mono.empty();
    }

    public Mono<Void> updateAttributes(List<ImporMeasurementDTO> data){
        for (ImporMeasurementDTO item : data){
            String internalCode = item.getItemId();
            Products product = productsRepository.getProduct(internalCode, ProductInterface.DYN.name())
                    .orElseGet(() -> {
                        Products newProduct = new Products();
                        newProduct.setItemId(internalCode);
                        newProduct.setPartNumber(item.getPartNumber());
                        newProduct.setCreatedAt(Instant.now());
                        newProduct.setUpdatedAt(Instant.now());
                        newProduct.setSkipNull("TRUE");
                        newProduct.setInterfaceId(ProductInterface.DYN.name());
                        newProduct.setDataAreaId(DataArea.MSB.name());
                        newProduct.setInterfaceRefRecId(4L);
                        newProduct.setCompanyRefRecId(1L);
                        newProduct.setProductCondition("new_new");
                        newProduct.setDangerGoodsRegulation("not_applicable");
                        newProduct.setCountryOfOrigin("MX");

                        return productsRepository.save(newProduct);
                    });
            Optional<ProductAttributeValues> itemId = productAttributeValueRepository.getAttribute(internalCode, "ITEM_ID");
            if (itemId.isEmpty()){
                ProductAttributeValues newAttribute = new ProductAttributeValues();
                newAttribute.setItemId(internalCode);
                newAttribute.setInterfaceId(ProductInterface.DYN.name());
                newAttribute.setProductAttributeId("ITEM_ID");
                newAttribute.setValue(internalCode);
                newAttribute.setSkipNull("TRUE");
                newAttribute.setCreatedAt(Instant.now());
                newAttribute.setUpdatedAt(Instant.now());
                newAttribute.setDataAreaId(DataArea.MSB.name());
                newAttribute.setInterfaceRefRecId(4L);
                newAttribute.setProductAttributeRefRecId(1L);
                newAttribute.setCompanyRefRecId(1L);
                newAttribute.setProductRefRecId(product.getId());

                productAttributeValueRepository.save(newAttribute);
            }

            Optional<ProductAttributeValues> partNumber = productAttributeValueRepository.getAttribute(internalCode, "PART_NUMBER");
            if (partNumber.isEmpty()){
                ProductAttributeValues newPartNumber = new ProductAttributeValues();
                newPartNumber.setItemId(internalCode);
                newPartNumber.setInterfaceId(ProductInterface.DYN.name());
                newPartNumber.setProductAttributeId("PART_NUMBER");
                newPartNumber.setValue(item.getPartNumber());
                newPartNumber.setSkipNull("TRUE");
                newPartNumber.setCreatedAt(Instant.now());
                newPartNumber.setUpdatedAt(Instant.now());
                newPartNumber.setDataAreaId(DataArea.MSB.name());
                newPartNumber.setInterfaceRefRecId(4L);
                newPartNumber.setProductAttributeRefRecId(3L);
                newPartNumber.setCompanyRefRecId(1L);
                newPartNumber.setProductRefRecId(product.getId());

                productAttributeValueRepository.save(newPartNumber);
            }

            if (item.getWidth().compareTo(BigDecimal.ZERO) > 0){
                Optional<ProductAttributeValues> width = productAttributeValueRepository.getAttribute(internalCode, "WIDTH");
                if (width.isEmpty()){
                    ProductAttributeValues newAttributeWidth = new ProductAttributeValues();
                    newAttributeWidth.setItemId(internalCode);
                    newAttributeWidth.setInterfaceId(ProductInterface.DYN.name());
                    newAttributeWidth.setProductAttributeId("WIDTH");
                    newAttributeWidth.setValue(item.getWidth().toString());
                    newAttributeWidth.setSkipNull("TRUE");
                    newAttributeWidth.setCreatedAt(Instant.now());
                    newAttributeWidth.setUpdatedAt(Instant.now());
                    newAttributeWidth.setDataAreaId(DataArea.MSB.name());
                    newAttributeWidth.setInterfaceRefRecId(4L);
                    newAttributeWidth.setProductAttributeRefRecId(13L);
                    newAttributeWidth.setCompanyRefRecId(1L);
                    newAttributeWidth.setProductRefRecId(product.getId());

                    productAttributeValueRepository.save(newAttributeWidth);
                } else {
                    ProductAttributeValues attributeWidth = width.get();
                    attributeWidth.setValue(item.getWidth().toString());

                    productAttributeValueRepository.save(attributeWidth);
                }
                System.out.println("Updated width item: " + internalCode);
            }

            if (item.getHeight().compareTo(BigDecimal.ZERO) > 0){
                Optional<ProductAttributeValues> height = productAttributeValueRepository.getAttribute(internalCode, "HEIGHT");
                if (height.isEmpty()){
                    ProductAttributeValues newAttributeHeight = new ProductAttributeValues();
                    newAttributeHeight.setItemId(internalCode);
                    newAttributeHeight.setInterfaceId(ProductInterface.DYN.name());
                    newAttributeHeight.setProductAttributeId("HEIGHT");
                    newAttributeHeight.setValue(item.getHeight().toString());
                    newAttributeHeight.setSkipNull("TRUE");
                    newAttributeHeight.setCreatedAt(Instant.now());
                    newAttributeHeight.setUpdatedAt(Instant.now());
                    newAttributeHeight.setDataAreaId(DataArea.MSB.name());
                    newAttributeHeight.setInterfaceRefRecId(4L);
                    newAttributeHeight.setProductAttributeRefRecId(12L);
                    newAttributeHeight.setCompanyRefRecId(1L);
                    newAttributeHeight.setProductRefRecId(product.getId());

                    productAttributeValueRepository.save(newAttributeHeight);
                } else {
                    ProductAttributeValues attributeHeight = height.get();
                    attributeHeight.setValue(item.getHeight().toString());

                    productAttributeValueRepository.save(attributeHeight);
                }
                System.out.println("Updated height item: " + internalCode);
            }

            if (item.getLength().compareTo(BigDecimal.ZERO) > 0){
                Optional<ProductAttributeValues> length = productAttributeValueRepository.getAttribute(internalCode, "LENGTH");
                if (length.isEmpty()){
                    ProductAttributeValues newAttributeHeight = new ProductAttributeValues();
                    newAttributeHeight.setItemId(internalCode);
                    newAttributeHeight.setInterfaceId(ProductInterface.DYN.name());
                    newAttributeHeight.setProductAttributeId("LENGTH");
                    newAttributeHeight.setValue(item.getLength().toString());
                    newAttributeHeight.setSkipNull("TRUE");
                    newAttributeHeight.setCreatedAt(Instant.now());
                    newAttributeHeight.setUpdatedAt(Instant.now());
                    newAttributeHeight.setDataAreaId(DataArea.MSB.name());
                    newAttributeHeight.setInterfaceRefRecId(4L);
                    newAttributeHeight.setProductAttributeRefRecId(11L);
                    newAttributeHeight.setCompanyRefRecId(1L);
                    newAttributeHeight.setProductRefRecId(product.getId());

                    productAttributeValueRepository.save(newAttributeHeight);
                } else {
                    ProductAttributeValues attributeLength = length.get();
                    attributeLength.setValue(item.getLength().toString());

                    productAttributeValueRepository.save(attributeLength);
                }
                System.out.println("Updated length item: " + internalCode);
            }

            if (item.getWeight().compareTo(BigDecimal.ZERO) > 0){
                Optional<ProductAttributeValues> weight = productAttributeValueRepository.getAttribute(internalCode, "WEIGHT");
                BigDecimal factorConversion = new BigDecimal("1");
                BigDecimal weightValue = item.getWeight().multiply(factorConversion).setScale(2, RoundingMode.HALF_UP);
                if (weight.isEmpty()){
                    ProductAttributeValues newAttributeHeight = new ProductAttributeValues();
                    newAttributeHeight.setItemId(internalCode);
                    newAttributeHeight.setInterfaceId(ProductInterface.DYN.name());
                    newAttributeHeight.setProductAttributeId("WEIGHT");
                    newAttributeHeight.setValue(weightValue.toString());
                    newAttributeHeight.setSkipNull("TRUE");
                    newAttributeHeight.setCreatedAt(Instant.now());
                    newAttributeHeight.setUpdatedAt(Instant.now());
                    newAttributeHeight.setDataAreaId(DataArea.MSB.name());
                    newAttributeHeight.setInterfaceRefRecId(4L);
                    newAttributeHeight.setProductAttributeRefRecId(7L);
                    newAttributeHeight.setCompanyRefRecId(1L);
                    newAttributeHeight.setProductRefRecId(product.getId());

                    productAttributeValueRepository.save(newAttributeHeight);
                } else {
                    ProductAttributeValues attributeWeight = weight.get();
                    attributeWeight.setValue(weightValue.toString());

                    productAttributeValueRepository.save(attributeWeight);
                }
                System.out.println("Updated weight item: " + internalCode);
            }

            if (item.getDiameter().compareTo(BigDecimal.ZERO) > 0){
                Optional<ProductAttributeValues> diameter = productAttributeValueRepository.getAttribute(internalCode, "DIAMETER");
                if (diameter.isEmpty()){
                    ProductAttributeValues newAttributeDiameter = new ProductAttributeValues();
                    newAttributeDiameter.setItemId(internalCode);
                    newAttributeDiameter.setInterfaceId(ProductInterface.DYN.name());
                    newAttributeDiameter.setProductAttributeId("DIAMETER");
                    newAttributeDiameter.setValue(item.getDiameter().toString());
                    newAttributeDiameter.setSkipNull("TRUE");
                    newAttributeDiameter.setCreatedAt(Instant.now());
                    newAttributeDiameter.setUpdatedAt(Instant.now());
                    newAttributeDiameter.setDataAreaId(DataArea.MSB.name());
                    newAttributeDiameter.setInterfaceRefRecId(4L);
                    newAttributeDiameter.setProductAttributeRefRecId(21L);
                    newAttributeDiameter.setCompanyRefRecId(1L);
                    newAttributeDiameter.setProductRefRecId(product.getId());

                    productAttributeValueRepository.save(newAttributeDiameter);
                } else {
                    ProductAttributeValues attributeDiameter = diameter.get();
                    attributeDiameter.setValue(item.getDiameter().toString());

                    productAttributeValueRepository.save(attributeDiameter);
                }
                System.out.println("Updated diameter item: " + internalCode);
            }
        }

        return Mono.empty();
    }

    public Mono<Void> importImages(List<ImportImageDTO> data){
        Set<String> pendingItems = new HashSet<>();
        for (ImportImageDTO imageData : data){
            String itemId = imageData.getItemId();
            Optional<Products> product = productsRepository.getProduct(itemId, ProductInterface.DYN.name());

            if (product.isPresent()){
                Products productData = product.get();
                ProductImage image = imageRepository.getImage(itemId, imageData.getFilename(), ProductInterface.DYN.name())
                        .orElseGet(ProductImage::new);
                image.setItemId(itemId);
                image.setPartNumber(imageData.getPartNumber());
                image.setItemName(productData.getProductName());
                image.setImageUrl(imageData.getFileUrl());
                image.setBlobName(imageData.getFilename());
                image.setUpdatedAt(Instant.now());
                image.setCreatedAt(Instant.now());
                image.setInterfaceId(ProductInterface.DYN.name());
                image.setInterfaceRefRecId(4L);
                image.setDataAreaId(DataArea.MSB.name());
                image.setCompanyRefRecId(1L);
                image.setImageNumber(imageData.getConsecutivo());
                image.setActive(true);
                image.setPriority(1L);

                if (image.getRecId() == null){
                    System.out.println("New image for: " + itemId);
                }

                imageRepository.save(image);
                imageCorrection(itemId);
                pendingItems.add(itemId);
            }
        }

        for (String pending : pendingItems){
            ProductPendings newPending = pendingsRepository.findByInternalCode(pending)
                    .orElseGet(ProductPendings::new);
            newPending.setInternalCode(pending);

            pendingsRepository.save(newPending);
        }

        System.out.println("Image upload ended");

        return Mono.empty();
    }

    private void imageCorrection(String itemId){
        List<ProductImage> oldImages = imageRepository.getOldImages(itemId);

        for (ProductImage image : oldImages){
            image.setActive(false);
            imageRepository.save(image);
        }
    }
}
