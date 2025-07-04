package com.vegusa.middleware.service;

import com.vegusa.middleware.constants.AttributeParameters;
import com.vegusa.middleware.constants.DataArea;
import com.vegusa.middleware.constants.ProductInterface;
import com.vegusa.middleware.dto.ImportSeoDTO;
import com.vegusa.middleware.entity.*;
import com.vegusa.middleware.repository.*;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Mono;

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
}
