package com.vegusa.middleware.entity;

import jakarta.persistence.*;

@Entity
@Table(name = "SyncItemPrice")
public class SyncItemPrice {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId", nullable = false)
    private Long id;

    @Column(name = "ProductVersionId", length = 100)
    private String productVersionId;

    @Column(name = "gross", length = 20)
    private String gross;

    @Column(name = "oldPriceWithDiscount", length = 20)
    private String oldPriceWithDiscount;

    @Column(name = "success", length = 10)
    private String success;

    @Column(name = "tax", length = 20)
    private String tax;

    @Column(name = "oldGross", length = 20)
    private String oldGross;

    @Column(name = "oldnet", length = 20)
    private String oldnet;

    @Column(name = "net", length = 20)
    private String net;

    @Column(name = "priceWithDiscount", length = 20)
    private String priceWithDiscount;

    @Column(name = "oldtax", length = 20)
    private String oldtax;

    @Column(name = "PriceListId", length = 100)
    private String priceListId;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumns({
            @JoinColumn(name = "CompanyRefRecId", referencedColumnName = "RecId", nullable = false),
            @JoinColumn(name = "DataAreaId", referencedColumnName = "DataAreaId", nullable = false)
    })
    private Company company;

    public Long getId() {
        return id;
    }

    public void setId(Long id) {
        this.id = id;
    }

    public String getProductVersionId() {
        return productVersionId;
    }

    public void setProductVersionId(String productVersionId) {
        this.productVersionId = productVersionId;
    }

    public String getGross() {
        return gross;
    }

    public void setGross(String gross) {
        this.gross = gross;
    }

    public String getOldPriceWithDiscount() {
        return oldPriceWithDiscount;
    }

    public void setOldPriceWithDiscount(String oldPriceWithDiscount) {
        this.oldPriceWithDiscount = oldPriceWithDiscount;
    }

    public String getSuccess() {
        return success;
    }

    public void setSuccess(String success) {
        this.success = success;
    }

    public String getTax() {
        return tax;
    }

    public void setTax(String tax) {
        this.tax = tax;
    }

    public String getOldGross() {
        return oldGross;
    }

    public void setOldGross(String oldGross) {
        this.oldGross = oldGross;
    }

    public String getOldnet() {
        return oldnet;
    }

    public void setOldnet(String oldnet) {
        this.oldnet = oldnet;
    }

    public String getNet() {
        return net;
    }

    public void setNet(String net) {
        this.net = net;
    }

    public String getPriceWithDiscount() {
        return priceWithDiscount;
    }

    public void setPriceWithDiscount(String priceWithDiscount) {
        this.priceWithDiscount = priceWithDiscount;
    }

    public String getOldtax() {
        return oldtax;
    }

    public void setOldtax(String oldtax) {
        this.oldtax = oldtax;
    }

    public String getPriceListId() {
        return priceListId;
    }

    public void setPriceListId(String priceListId) {
        this.priceListId = priceListId;
    }

    public Company getCompany() {
        return company;
    }

    public void setCompany(Company company) {
        this.company = company;
    }

}